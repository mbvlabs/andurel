package layout

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	// ProjectTomlName is the human-edited V2 project manifest.
	ProjectTomlName = "andurel.toml"
	// ProjectLockName is the CLI-owned digest lock (TOML).
	ProjectLockName = "andurel.lock"
)

// projectTomlFile is the on-disk andurel.toml document.
type projectTomlFile struct {
	SchemaVersion int                 `toml:"schemaVersion"`
	Version       string              `toml:"version"`
	Project       projectTomlProject  `toml:"project"`
	Database      projectTomlDatabase `toml:"database"`
	Tools         map[string]string   `toml:"tools"`
}

type projectTomlProject struct {
	Name                     string `toml:"name"`
	Inertia                  string `toml:"inertia,omitempty"`
	JavaScriptPackageManager string `toml:"javascriptPackageManager,omitempty"`
	JavaScriptSSRRuntime     string `toml:"javascriptSSRRuntime,omitempty"`
}

type projectTomlDatabase struct {
	Engine   string `toml:"engine"`
	NullType string `toml:"nullType"`
}

// projectLockFile is the on-disk digest-only andurel.lock document.
type projectLockFile struct {
	Hashes []projectLockHash `toml:"hashes"`
}

type projectLockHash struct {
	Tool     string `toml:"tool"`
	Version  string `toml:"version"`
	Platform string `toml:"platform"`
	SHA256   string `toml:"sha256"`
}

func projectTomlPath(root string) string {
	return filepath.Join(root, ProjectTomlName)
}

func projectLockPath(root string) string {
	return filepath.Join(root, ProjectLockName)
}

func encodeProjectToml(doc projectTomlFile) ([]byte, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data, nil
}

func encodeProjectLock(doc projectLockFile) ([]byte, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data, nil
}

func lockToProjectToml(lock *AndurelLock) (projectTomlFile, error) {
	if lock == nil {
		return projectTomlFile{}, fmt.Errorf("lock is required")
	}

	project := projectTomlProject{}
	if lock.ScaffoldConfig != nil {
		project = projectTomlProject{
			Name:                     lock.ScaffoldConfig.ProjectName,
			Inertia:                  lock.ScaffoldConfig.Inertia,
			JavaScriptPackageManager: lock.ScaffoldConfig.PackageManager(),
			JavaScriptSSRRuntime:     lock.ScaffoldConfig.JavaScriptSSRRuntime,
		}
	}

	database := projectTomlDatabase{
		Engine:   DatabaseEnginePostgreSQL,
		NullType: NullTypePGType,
	}
	if lock.DatabaseConfig != nil {
		database.Engine = lock.DatabaseConfig.Engine
		database.NullType = lock.DatabaseConfig.NullType
	}

	tools := make(map[string]string, len(lock.Tools))
	for name, tool := range lock.Tools {
		if tool == nil {
			continue
		}
		tools[name] = tool.Version
	}

	return projectTomlFile{
		SchemaVersion: lock.SchemaVersion,
		Version:       lock.Version,
		Project:       project,
		Database:      database,
		Tools:         tools,
	}, nil
}

func lockToProjectLock(lock *AndurelLock) projectLockFile {
	doc := projectLockFile{Hashes: make([]projectLockHash, 0)}
	if lock == nil {
		return doc
	}
	names := make([]string, 0, len(lock.Tools))
	for name := range lock.Tools {
		names = append(names, name)
	}
	// Stable write order for goldens.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for _, name := range names {
		tool := lock.Tools[name]
		if tool == nil || tool.Download == nil {
			continue
		}
		platforms := append([]string{}, requiredChecksumPlatforms...)
		for _, platform := range platforms {
			digest, ok := tool.Download.SHA256[platform]
			if !ok || digest == "" {
				continue
			}
			doc.Hashes = append(doc.Hashes, projectLockHash{
				Tool:     name,
				Version:  tool.Version,
				Platform: platform,
				SHA256:   digest,
			})
		}
	}
	return doc
}

func assembleLockFromProjectFiles(tomlDoc projectTomlFile, digestDoc projectLockFile) (*AndurelLock, error) {
	lock := &AndurelLock{
		SchemaVersion: tomlDoc.SchemaVersion,
		Version:       tomlDoc.Version,
		Tools:         make(map[string]*Tool),
		DatabaseConfig: &DatabaseConfig{
			Engine:   tomlDoc.Database.Engine,
			NullType: tomlDoc.Database.NullType,
		},
	}
	if strings.TrimSpace(tomlDoc.Project.Name) != "" ||
		tomlDoc.Project.Inertia != "" ||
		tomlDoc.Project.JavaScriptPackageManager != "" ||
		tomlDoc.Project.JavaScriptSSRRuntime != "" {
		lock.ScaffoldConfig = &ScaffoldConfig{
			ProjectName:              tomlDoc.Project.Name,
			Inertia:                  tomlDoc.Project.Inertia,
			JavaScriptPackageManager: tomlDoc.Project.JavaScriptPackageManager,
			JavaScriptSSRRuntime:     tomlDoc.Project.JavaScriptSSRRuntime,
		}
	}

	digests := indexProjectLockHashes(digestDoc)
	for name, version := range tomlDoc.Tools {
		tool, err := toolFromProjectEntry(name, version, digests)
		if err != nil {
			return nil, err
		}
		lock.Tools[name] = tool
	}

	migrateLegacyScaffoldConfig(lock.ScaffoldConfig)
	migrateLegacyDatabaseConfig(lock)
	if err := validateSchema1Lock(lock); err != nil {
		return nil, err
	}
	return lock, nil
}

func indexProjectLockHashes(doc projectLockFile) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, hash := range doc.Hashes {
		key := hash.Tool + "@" + hash.Version
		if out[key] == nil {
			out[key] = make(map[string]string)
		}
		out[key][hash.Platform] = hash.SHA256
	}
	return out
}

func toolFromProjectEntry(
	name, version string,
	digests map[string]map[string]string,
) (*Tool, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("tool %q version is required", name)
	}

	var tool *Tool
	for _, goTool := range DefaultGoTools {
		if goTool.Name == name {
			tool = NewGoTool(name, extractRepo(goTool.Source), version)
			break
		}
	}
	if tool == nil {
		tool = NewBinaryTool(name, version)
	}
	if tool.Download == nil {
		if spec, ok := GetDefaultToolDownload(name); ok {
			tool.Download = &ToolDownload{
				URLTemplate: spec.URLTemplate,
				Archive:     spec.Archive,
				BinaryName:  spec.BinaryName,
				SHA256:      map[string]string{},
			}
		}
	}
	if tool.VersionCheck == nil {
		if vc, ok := GetDefaultToolVersionCheck(name); ok {
			tool.VersionCheck = vc
		}
	}

	key := name + "@" + version
	platformDigests, hasDigests := digests[key]
	if tool.Download != nil {
		if hasDigests {
			tool.Download.SHA256 = cloneStringMap(platformDigests)
		} else if len(tool.Download.SHA256) == 0 {
			if spec, ok := getDefaultToolDownloadForVersion(name, version); ok {
				tool.Download.SHA256 = cloneStringMap(spec.SHA256)
			}
		}
	}

	if hasCompleteToolDigests(tool) {
		return tool, nil
	}

	if hasDigests {
		// Digests were recorded for a non-catalog (or non-default-version) tool.
		tool.Path = ""
		tool.Download = &ToolDownload{
			URLTemplate: "https://example.invalid/" + name + "/{{version}}",
			Archive:     "binary",
			BinaryName:  name,
			SHA256:      cloneStringMap(platformDigests),
		}
		if tool.VersionCheck == nil {
			tool.VersionCheck = &VersionCheck{Args: []string{"--version"}}
		}
		return tool, nil
	}

	// Path-managed tools are not fully encoded in V2; reconstruct a local path.
	tool.Download = nil
	tool.Path = filepath.Join("bin", name)
	if tool.VersionCheck == nil {
		tool.VersionCheck = &VersionCheck{Args: []string{"--version"}}
	}
	return tool, nil
}

func hasCompleteToolDigests(tool *Tool) bool {
	if tool == nil || tool.Download == nil {
		return false
	}
	for _, platform := range requiredChecksumPlatforms {
		digest := tool.Download.SHA256[platform]
		if !sha256Pattern.MatchString(digest) {
			return false
		}
	}
	return true
}

func looksLikeJSONLock(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func rejectLegacyJSONLock(root string, lockData []byte) error {
	tomlPath := projectTomlPath(root)
	if _, err := os.Stat(tomlPath); err == nil {
		return nil
	}
	if looksLikeJSONLock(lockData) {
		return fmt.Errorf(
			"V2 projects require %s; found legacy JSON %s without a manifest — recreate with andurel new or migrate manually",
			ProjectTomlName,
			ProjectLockName,
		)
	}
	return nil
}

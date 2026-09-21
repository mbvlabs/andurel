package layout

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/layout/versions"
	toml "github.com/pelletier/go-toml/v2"
)

// AndurelLock is the serialized project lock file for tools.
type AndurelLock struct {
	SchemaVersion  int              `json:"schemaVersion"`
	Version        string           `json:"version"`
	Tools          map[string]*Tool `json:"tools"`
	ScaffoldConfig *ScaffoldConfig  `json:"scaffoldConfig,omitempty"`
	DatabaseConfig *DatabaseConfig  `json:"databaseConfig,omitempty"`
}

const (
	// NullTypePGType is the default nullable strategy for pgx/v5 (pgtype.*).
	NullTypePGType = "pgtype.Null"
	// NullTypePointer emits *T for nullable columns instead of pgtype wrappers.
	NullTypePointer = "pointer"
	// DatabaseEnginePostgreSQL is the only supported SQL engine.
	DatabaseEnginePostgreSQL = "postgresql"
)

// DatabaseConfig records database engine and generation settings.
type DatabaseConfig struct {
	Engine   string `json:"engine"`
	NullType string `json:"nullType"`
}

// ScaffoldConfig records the options used to create a project.
type ScaffoldConfig struct {
	ProjectName string `json:"projectName"`
	// Database is deprecated; engine lives on DatabaseConfig. Kept for reading
	// older locks before migrateLegacyDatabaseConfig runs.
	Database                 string `json:"database,omitempty"`
	Inertia                  string `json:"inertia,omitempty"`
	JavaScriptPackageManager string `json:"javascriptPackageManager,omitempty"`
	JavaScriptRuntime        string `json:"javascriptRuntime,omitempty"` // Deprecated: use JavaScriptPackageManager.
	JavaScriptSSRRuntime     string `json:"javascriptSSRRuntime,omitempty"`
}

// DatabaseEngine returns the configured SQL engine after legacy migration.
func (l *AndurelLock) DatabaseEngine() string {
	if l == nil || l.DatabaseConfig == nil {
		return ""
	}
	return l.DatabaseConfig.Engine
}

// PackageManager returns the configured JavaScript package manager. Locks from
// before V2 used javascriptRuntime for this value.
func (config *ScaffoldConfig) PackageManager() string {
	if config == nil {
		return ""
	}
	if config.JavaScriptPackageManager != "" {
		return config.JavaScriptPackageManager
	}
	return config.JavaScriptRuntime
}

// ToolDownload describes how to download a managed tool binary.
type ToolDownload struct {
	URLTemplate string            `json:"urlTemplate"`
	Archive     string            `json:"archive,omitempty"`
	BinaryName  string            `json:"binaryName,omitempty"`
	SHA256      map[string]string `json:"sha256"`
}

// VersionCheck describes how to check an installed tool version.
type VersionCheck struct {
	Args []string `json:"args"`
	// Regexp extracts a version from command output. When omitted, Andurel uses
	// v?MAJOR.MINOR.PATCH with optional prerelease or build metadata.
	Regexp string `json:"regexp,omitempty"`
}

// Tool records the desired version and source for a managed tool.
type Tool struct {
	Version      string        `json:"version,omitempty"`
	Source       string        `json:"source,omitempty"`
	Path         string        `json:"path,omitempty"`
	Download     *ToolDownload `json:"download,omitempty"`
	VersionCheck *VersionCheck `json:"versionCheck,omitempty"`
}

var defaultToolVersionChecks = map[string]VersionCheck{
	"narsilc":     {Args: []string{"version"}},
	"templ":       {Args: []string{"--version"}},
	"goose":       {Args: []string{"--version"}},
	"mailpit":     {Args: []string{"version", "--no-release-check"}},
	"usql":        {Args: []string{"--version"}},
	"dblab":       {Args: []string{"--version"}},
	"shadowfax":   {Args: []string{"--version"}},
	"tailwindcli": {Args: []string{"--version"}},
}

const defaultVersionCheckRegexp = `v?([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)`

var defaultToolVersions = map[string]string{
	"narsilc":     versions.Narsilc,
	"templ":       versions.Templ,
	"goose":       versions.Goose,
	"mailpit":     versions.Mailpit,
	"usql":        versions.Usql,
	"dblab":       versions.Dblab,
	"shadowfax":   versions.Shadowfax,
	"tailwindcli": versions.TailwindCLI,
}

var defaultToolDownloads = map[string]ToolDownload{
	"narsilc": {
		URLTemplate: "https://github.com/mbvlabs/narsilc/releases/download/{{version}}/narsilc_{{version_no_v}}_{{os}}_{{arch}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "narsilc",
		SHA256: map[string]string{
			"linux/amd64":  "0b57f97ecf4813bc60e361cd5cd5dc806b41bb434defd7cb399754f11d7c1c66",
			"linux/arm64":  "9d30975cea87055225d825772796b2faf31755a97ce10e6af2bf499fe08b4e18",
			"darwin/amd64": "1b2da14468a5e4b11b4acd18b6748fe9c0c7de897782f84253ad09c268241dc8",
			"darwin/arm64": "b8d9f53701fd9b3f3e428ad8d9632e9875d21dd8c3463a1807c93618df4a1994",
		},
	},
	"templ": {
		URLTemplate: "https://github.com/a-h/templ/releases/download/{{version}}/templ_{{os_capitalized}}_{{arch_x86_64}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "templ",
		SHA256: map[string]string{
			"linux/amd64":  "d1e726e8e78a6cf7e1e72ce3746f30fd94ec0eba10be1abab02208a41efc9aa5",
			"linux/arm64":  "8d38728fa82c0ee568d2ae1ce0720963402d384dd59d4c76bcdbb38d581c815c",
			"darwin/amd64": "f1522f2558081335584fd4fb67d329d02a9ae6e83dd88b14cae1ad84c770e5c0",
			"darwin/arm64": "f391943e3e49ece301f90c2283c7f9e629081f18b0b3ab6b48cb4b87ad94b206",
		},
	},
	"goose": {
		URLTemplate: "https://github.com/pressly/goose/releases/download/{{version}}/goose_{{os}}_{{arch_x86_64}}",
		Archive:     "binary",
		BinaryName:  "goose",
		SHA256: map[string]string{
			"linux/amd64":  "c5f1e5cd3b8e5da05592c2714b079d78ec846ddc7ec1f70d474c0449e79f6ab4",
			"linux/arm64":  "638af56b2ed33ff33cc3f30447f447b1c8e5894c6252fbda1e459adec94ba0fe",
			"darwin/amd64": "24f3b4dc3c792a7afba348de12245af8b4199cfd0ac6a673f7ab483b2ad48f08",
			"darwin/arm64": "d7867b7a9d1117024b17afdb043c573909e595d2c29dcee13bd96c341eb98ff2",
		},
	},
	"mailpit": {
		URLTemplate: "https://github.com/axllent/mailpit/releases/download/{{version}}/mailpit-{{os}}-{{arch}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "mailpit",
		SHA256: map[string]string{
			"linux/amd64":  "63b113aa9748adf7091b649ebe02693f99a459000cbe415faa6679f4b39f82cf",
			"linux/arm64":  "b159574f32e527f34624e5683f79859258360179268a8fac0f3030f74ca6bb96",
			"darwin/amd64": "6aabc9576350b98eed9d6d96427fec092361415c6a7c094842cba5773716a5b3",
			"darwin/arm64": "05b92a4b804c34b0f6e665a482a1141be64256f500ecf23a204c2084a27a248b",
		},
	},
	"usql": {
		URLTemplate: "https://github.com/xo/usql/releases/download/{{version}}/usql-{{version_no_v}}-{{os}}-{{arch}}.tar.bz2",
		Archive:     "tar.bz2",
		BinaryName:  "usql",
		SHA256: map[string]string{
			"linux/amd64":  "78bd9b221e223d7a954d41f51e9eca98bdd94b401618367ba0f3887abebd44fc",
			"linux/arm64":  "ccad89d6f4c67a9bf595df0aa8a550e0a9e3d6a6f9356356ba1e164e311335e4",
			"darwin/amd64": "60feb0d73b2e29e4ecf6b36bf34b23c230d8233c778893b0b56811036d16d17e",
			"darwin/arm64": "cee9fd2117c17fd622fe0df37b334c356b80cf48d6e9bb164084830ae4ce6d05",
		},
	},
	"dblab": {
		URLTemplate: "https://github.com/danvergara/dblab/releases/download/{{version}}/dblab_{{version_no_v}}_{{os}}_{{arch}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "dblab",
		SHA256: map[string]string{
			"linux/amd64":  "5672260f7230cda2a8a464480d80a5c08fba5e48ccc637cd709ade2c19bd1509",
			"linux/arm64":  "516487b248472edac79bd3a0c6a5b3a78a4bbef5990876bb4b9a1176c3252b1b",
			"darwin/amd64": "9070295d6f75e9cd5c015c3624c4fdf081711c16d44775d87aaeff44febe086a",
			"darwin/arm64": "99322f9e494a4b17a65d1c3618c7cad5694387325749be82255aa6afa2c0cdf5",
		},
	},
	"shadowfax": {
		URLTemplate: "https://github.com/mbvlabs/shadowfax/releases/download/{{version}}/shadowfax-{{os}}-{{arch}}",
		Archive:     "binary",
		BinaryName:  "shadowfax",
		SHA256: map[string]string{
			"linux/amd64":  "cecdcb89dfb292af27323f8089f61fbe0b3c4d63e3645acfadbcc3fb5c580b10",
			"linux/arm64":  "78bd369eff62794c023768c23a9c94383adefffa04e598c0fbad967a0817be16",
			"darwin/amd64": "92a9230740085aebbc7908fb4f22677860dfdb27a9ebaee39f914ffcdebb56b0",
			"darwin/arm64": "9d3203b315bd3e8d2cd1f8d7d11b6356be6fec551235794f6eb755c3d3aa3a9d",
		},
	},
	"tailwindcli": {
		URLTemplate: "https://github.com/tailwindlabs/tailwindcss/releases/download/{{version}}/tailwindcss-{{os_tailwind}}-{{arch_tailwind}}",
		Archive:     "binary",
		BinaryName:  "tailwindcli",
		SHA256: map[string]string{
			"linux/amd64":  "5036c4fb4328e0bcdbb6065c70d8ac9452e0d4c947113a788a8f94fd390425c1",
			"linux/arm64":  "394ddccc2402cfa3abd97dfba56f3587781a3d6e6ce66e65ceada14beb7664b8",
			"darwin/amd64": "cef8f110471e889c3c4409055cf8aff33076f58a081867b0dfc6534b290bfbb0",
			"darwin/arm64": "b800b0659dc64b9f03ede5660244d9415d777d5739ae2889280877ca37be742a",
		},
	},
}

// NewAndurelLock creates a lock file model for a version with default
// databaseConfig (postgresql + pgtype.Null).
func NewAndurelLock(version string) *AndurelLock {
	return &AndurelLock{
		SchemaVersion: 1,
		Version:       version,
		Tools:         make(map[string]*Tool),
		DatabaseConfig: &DatabaseConfig{
			Engine:   DatabaseEnginePostgreSQL,
			NullType: NullTypePGType,
		},
	}
}

// GetDefaultToolVersionCheck returns the built-in version check for a tool.
func GetDefaultToolVersionCheck(name string) (*VersionCheck, bool) {
	vc, ok := defaultToolVersionChecks[name]
	if !ok {
		return nil, false
	}
	return &VersionCheck{
		Args:   append([]string{}, vc.Args...),
		Regexp: vc.Regexp,
	}, true
}

// GetDefaultToolDownload returns the built-in download spec for a tool.
func GetDefaultToolDownload(name string) (*ToolDownload, bool) {
	spec, ok := defaultToolDownloads[name]
	if !ok {
		return nil, false
	}

	return &ToolDownload{
		URLTemplate: spec.URLTemplate,
		Archive:     spec.Archive,
		BinaryName:  spec.BinaryName,
		SHA256:      cloneStringMap(spec.SHA256),
	}, true
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	maps.Copy(cloned, values)
	return cloned
}

// NewGoTool creates a managed Go-installed tool entry.
func NewGoTool(name, source, version string) *Tool {
	tool := &Tool{
		Source:  source,
		Version: version,
	}

	if spec, ok := getDefaultToolDownloadForVersion(name, version); ok {
		tool.Download = spec
	}

	if vc, ok := GetDefaultToolVersionCheck(name); ok {
		tool.VersionCheck = vc
	}

	return tool
}

// NewBinaryTool creates a managed downloaded binary tool entry.
func NewBinaryTool(name, version string) *Tool {
	tool := &Tool{Version: version}
	if spec, ok := getDefaultToolDownloadForVersion(name, version); ok {
		tool.Download = spec
	}
	if vc, ok := GetDefaultToolVersionCheck(name); ok {
		tool.VersionCheck = vc
	}
	return tool
}

// NewBuiltTool creates a managed project-built tool entry.
func NewBuiltTool(path, version string) *Tool {
	return &Tool{
		Path:         path,
		Version:      version,
		VersionCheck: &VersionCheck{Args: []string{"--version"}},
	}
}

func getDefaultToolDownloadForVersion(name, version string) (*ToolDownload, bool) {
	if defaultToolVersions[name] != version {
		return nil, false
	}
	return GetDefaultToolDownload(name)
}

// AddTool records a managed tool in the lock file.
func (l *AndurelLock) AddTool(name string, tool *Tool) {
	l.Tools[name] = tool
}

// EncodeProjectFiles returns the andurel.toml and andurel.lock encodings for l.
func (l *AndurelLock) EncodeProjectFiles() (tomlData, lockData []byte, err error) {
	if err := validateSchema1Lock(l); err != nil {
		return nil, nil, fmt.Errorf("failed to validate lock file: %w", err)
	}
	tomlDoc, err := lockToProjectToml(l)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build project manifest: %w", err)
	}
	tomlData, err = encodeProjectToml(tomlDoc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal %s: %w", ProjectTomlName, err)
	}
	lockData, err = encodeProjectLock(lockToProjectLock(l))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal %s: %w", ProjectLockName, err)
	}
	return tomlData, lockData, nil
}

// WriteLockFile writes andurel.toml (manifest) and andurel.lock (digests).
func (l *AndurelLock) WriteLockFile(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	tomlData, lockData, err := l.EncodeProjectFiles()
	if err != nil {
		return err
	}
	if err := os.WriteFile(projectTomlPath(absTargetDir), tomlData, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", ProjectTomlName, err)
	}
	if err := os.WriteFile(projectLockPath(absTargetDir), lockData, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", ProjectLockName, err)
	}

	return nil
}

// Sync downloads missing managed tools and rewrites the lock file.
func (l *AndurelLock) Sync(targetDir string, silent bool) error {
	if err := validateSchema1Lock(l); err != nil {
		return fmt.Errorf("invalid lock file: %w", err)
	}
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	if err := l.WriteLockFile(absTargetDir); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	binDir := filepath.Join(absTargetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	for name, tool := range l.Tools {
		binPath := filepath.Join(binDir, name)

		if _, err := os.Stat(binPath); err == nil {
			actual, versionErr := installedToolVersion(binPath, tool.VersionCheck)
			if versionErr == nil && lockVersionsMatch(tool.Version, actual) {
				continue
			}
		}

		candidate, err := os.CreateTemp(binDir, ".andurel-candidate-*")
		if err != nil {
			return fmt.Errorf("failed to create candidate path for %s: %w", name, err)
		}
		candidatePath := candidate.Name()
		if err := candidate.Close(); err != nil {
			_ = os.Remove(candidatePath)
			return fmt.Errorf("failed to close candidate path for %s: %w", name, err)
		}
		if err := os.Remove(candidatePath); err != nil {
			return fmt.Errorf("failed to prepare candidate path for %s: %w", name, err)
		}

		if err := downloadToolBinary(name, tool, goos, goarch, candidatePath); err != nil {
			_ = os.Remove(candidatePath)
			return fmt.Errorf("failed to download %s: %w", name, err)
		}
		actual, err := installedToolVersion(candidatePath, tool.VersionCheck)
		if err != nil {
			_ = os.Remove(candidatePath)
			return fmt.Errorf("failed to verify downloaded %s: %w", name, err)
		}
		if !lockVersionsMatch(tool.Version, actual) {
			_ = os.Remove(candidatePath)
			return fmt.Errorf(
				"downloaded %s version %s does not match expected %s",
				name,
				actual,
				tool.Version,
			)
		}
		if err := os.Rename(candidatePath, binPath); err != nil {
			_ = os.Remove(candidatePath)
			return fmt.Errorf("failed to atomically replace %s: %w", name, err)
		}
	}

	return nil
}

func installedToolVersion(binaryPath string, check *VersionCheck) (string, error) {
	if check == nil || len(check.Args) == 0 {
		return "", fmt.Errorf("versionCheck.args is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binaryPath, check.Args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	expression := check.Regexp
	if expression == "" {
		expression = defaultVersionCheckRegexp
	}
	pattern, err := regexp.Compile(expression)
	if err != nil {
		return "", err
	}
	matches := pattern.FindStringSubmatch(string(output))
	if len(matches) == 0 {
		return "", fmt.Errorf("version output did not match configured expression")
	}
	for _, match := range matches[1:] {
		if match != "" {
			return match, nil
		}
	}
	return matches[0], nil
}

func lockVersionsMatch(expected, actual string) bool {
	return strings.TrimPrefix(
		strings.TrimSpace(expected),
		"v",
	) == strings.TrimPrefix(
		strings.TrimSpace(actual),
		"v",
	)
}

func downloadToolBinary(name string, tool *Tool, goos, goarch, destPath string) error {
	if tool == nil {
		return fmt.Errorf("tool configuration is nil")
	}

	if tool.Download != nil && tool.Download.URLTemplate != "" {
		platform := goos + "/" + goarch
		if !isSupportedChecksumPlatform(platform) {
			return fmt.Errorf("unsupported platform %s", platform)
		}
		digest := tool.Download.SHA256[platform]
		if digest == "" {
			return fmt.Errorf("missing SHA-256 digest for %s", platform)
		}
		archive := tool.Download.Archive
		if archive == "" {
			archive = "binary"
		}

		return cmds.DownloadVerifiedFromURLTemplate(
			name,
			tool.Version,
			tool.Download.URLTemplate,
			archive,
			tool.Download.BinaryName,
			goos,
			goarch,
			destPath,
			digest,
		)
	}

	if tool.Source != "" {
		return fmt.Errorf(
			"tool source downloads require explicit download metadata and SHA-256 digests",
		)
	}

	if name == "tailwindcli" {
		return fmt.Errorf(
			"tailwindcli downloads require explicit download metadata and SHA-256 digests",
		)
	}

	return fmt.Errorf("tool has no download metadata")
}

// ReadLockFile reads andurel.toml plus digest-only andurel.lock.
func ReadLockFile(targetDir string) (*AndurelLock, error) {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	lockPath := projectLockPath(absTargetDir)
	lockData, lockErr := os.ReadFile(lockPath)
	if lockErr == nil {
		if err := rejectLegacyJSONLock(absTargetDir, lockData); err != nil {
			return nil, err
		}
	}

	tomlPath := projectTomlPath(absTargetDir)
	tomlData, err := os.ReadFile(tomlPath)
	if err != nil {
		if lockErr == nil && looksLikeJSONLock(lockData) {
			return nil, fmt.Errorf(
				"V2 projects require %s; found legacy JSON %s without a manifest — recreate with andurel new or migrate manually",
				ProjectTomlName,
				ProjectLockName,
			)
		}
		return nil, fmt.Errorf("failed to read %s: %w", ProjectTomlName, err)
	}

	var tomlDoc projectTomlFile
	if err := toml.Unmarshal(tomlData, &tomlDoc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ProjectTomlName, err)
	}

	var digestDoc projectLockFile
	if lockErr != nil {
		if !os.IsNotExist(lockErr) {
			return nil, fmt.Errorf("failed to read %s: %w", ProjectLockName, lockErr)
		}
	} else {
		if looksLikeJSONLock(lockData) {
			return nil, fmt.Errorf(
				"%s must be the TOML digest lock; found legacy JSON — recreate with andurel new",
				ProjectLockName,
			)
		}
		if err := toml.Unmarshal(lockData, &digestDoc); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", ProjectLockName, err)
		}
	}

	lock, err := assembleLockFromProjectFiles(tomlDoc, digestDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble project files: %w", err)
	}
	return lock, nil
}

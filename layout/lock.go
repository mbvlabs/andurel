package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mbvlabs/andurel/layout/cmds"
)

type AndurelLock struct {
	Version        string                `json:"version"`
	Extensions     map[string]*Extension `json:"extensions,omitempty"`
	Tools          map[string]*Tool      `json:"tools"`
	ScaffoldConfig *ScaffoldConfig       `json:"scaffoldConfig,omitempty"`
}

type ScaffoldConfig struct {
	ProjectName  string   `json:"projectName"`
	Database     string   `json:"database"`
	CSSFramework string   `json:"cssFramework"`
	Extensions   []string `json:"extensions,omitempty"`
}

type Extension struct {
	AppliedAt string `json:"appliedAt"`
}

type ToolDownload struct {
	URLTemplate string `json:"urlTemplate"`
	Archive     string `json:"archive,omitempty"`
	BinaryName  string `json:"binaryName,omitempty"`
}

type Tool struct {
	Version  string        `json:"version,omitempty"`
	Module   string        `json:"module,omitempty"`
	Path     string        `json:"path,omitempty"`
	Download *ToolDownload `json:"download,omitempty"`
}

var defaultToolDownloads = map[string]ToolDownload{
	"templ": {
		URLTemplate: "https://github.com/a-h/templ/releases/download/{{version}}/templ_{{os_capitalized}}_{{arch_x86_64}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "templ",
	},
	"sqlc": {
		URLTemplate: "https://github.com/sqlc-dev/sqlc/releases/download/{{version}}/sqlc_{{version_no_v}}_{{os}}_{{arch}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "sqlc",
	},
	"goose": {
		URLTemplate: "https://github.com/pressly/goose/releases/download/{{version}}/goose_{{os}}_{{arch_x86_64}}",
		Archive:     "binary",
		BinaryName:  "goose",
	},
	"mailpit": {
		URLTemplate: "https://github.com/axllent/mailpit/releases/download/{{version}}/mailpit-{{os}}-{{arch}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "mailpit",
	},
	"usql": {
		URLTemplate: "https://github.com/xo/usql/releases/download/{{version}}/usql-{{version_no_v}}-{{os}}-{{arch}}.tar.bz2",
		Archive:     "tar.bz2",
		BinaryName:  "usql",
	},
	"dblab": {
		URLTemplate: "https://github.com/danvergara/dblab/releases/download/{{version}}/dblab_{{version_no_v}}_{{os}}_{{arch}}.tar.gz",
		Archive:     "tar.gz",
		BinaryName:  "dblab",
	},
	"shadowfax": {
		URLTemplate: "https://github.com/mbvlabs/shadowfax/releases/download/{{version}}/shadowfax-{{os}}-{{arch}}",
		Archive:     "binary",
		BinaryName:  "shadowfax",
	},
	"tailwindcli": {
		URLTemplate: "https://github.com/tailwindlabs/tailwindcss/releases/download/{{version}}/tailwindcss-{{os_tailwind}}-{{arch_tailwind}}",
		Archive:     "binary",
		BinaryName:  "tailwindcli",
	},
}

func NewAndurelLock(version string) *AndurelLock {
	return &AndurelLock{
		Version:    version,
		Extensions: make(map[string]*Extension),
		Tools:      make(map[string]*Tool),
	}
}

func GetDefaultToolDownload(name string) (*ToolDownload, bool) {
	spec, ok := defaultToolDownloads[name]
	if !ok {
		return nil, false
	}

	return &ToolDownload{
		URLTemplate: spec.URLTemplate,
		Archive:     spec.Archive,
		BinaryName:  spec.BinaryName,
	}, true
}

func NewGoTool(name, module, version string) *Tool {
	tool := &Tool{
		Module:  module,
		Version: version,
	}

	if spec, ok := GetDefaultToolDownload(name); ok {
		tool.Download = spec
	}

	return tool
}

func NewBinaryTool(name, version string) *Tool {
	tool := &Tool{Version: version}
	if spec, ok := GetDefaultToolDownload(name); ok {
		tool.Download = spec
	}
	return tool
}

func NewBuiltTool(path, version string) *Tool {
	return &Tool{
		Path:    path,
		Version: version,
	}
}

func (l *AndurelLock) AddTool(name string, tool *Tool) {
	l.Tools[name] = tool
}

func (l *AndurelLock) AddExtension(name, appliedAt string) {
	l.Extensions[name] = &Extension{
		AppliedAt: appliedAt,
	}
}

func (l *AndurelLock) WriteLockFile(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	lockPath := filepath.Join(absTargetDir, "andurel.lock")

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

func (l *AndurelLock) Sync(targetDir string, silent bool) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
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
			continue
		}

		if err := downloadToolBinary(name, tool, goos, goarch, binPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", name, err)
		}
	}

	if err := l.WriteLockFile(absTargetDir); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	return nil
}

func downloadToolBinary(name string, tool *Tool, goos, goarch, destPath string) error {
	if tool == nil {
		return fmt.Errorf("tool configuration is nil")
	}

	if tool.Download != nil && tool.Download.URLTemplate != "" {
		archive := tool.Download.Archive
		if archive == "" {
			archive = "binary"
		}

		return cmds.DownloadFromURLTemplate(
			name,
			tool.Version,
			tool.Download.URLTemplate,
			archive,
			tool.Download.BinaryName,
			goos,
			goarch,
			destPath,
		)
	}

	if tool.Module != "" {
		return cmds.DownloadGoTool(name, tool.Module, tool.Version, goos, goarch, destPath)
	}

	if name == "tailwindcli" {
		return cmds.DownloadTailwindCLI(tool.Version, goos, goarch, destPath)
	}

	return fmt.Errorf("tool has no download metadata")
}

func ReadLockFile(targetDir string) (*AndurelLock, error) {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	lockPath := filepath.Join(absTargetDir, "andurel.lock")

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	var lock AndurelLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse lock file: %w", err)
	}

	return &lock, nil
}

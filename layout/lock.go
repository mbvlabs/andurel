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
	Version          string                `json:"version"`
	Extensions       map[string]*Extension `json:"extensions,omitempty"`
	Tools            map[string]*Tool      `json:"tools"`
	FrameworkVersion string                `json:"frameworkVersion,omitempty"`
	ScaffoldConfig   *ScaffoldConfig       `json:"scaffoldConfig,omitempty"`
}

type ScaffoldConfig struct {
	ProjectName  string   `json:"projectName"`
	Repository   string   `json:"repository,omitempty"`
	Database     string   `json:"database"`
	CSSFramework string   `json:"cssFramework"`
	Extensions   []string `json:"extensions,omitempty"`
}

type Extension struct {
	AppliedAt string `json:"appliedAt"`
}

type Tool struct {
	Source  string `json:"source"`
	Version string `json:"version"`
	Module  string `json:"module,omitempty"`
	Path    string `json:"path,omitempty"`
}

func NewAndurelLock(version string) *AndurelLock {
	return &AndurelLock{
		Version:    version,
		Extensions: make(map[string]*Extension),
		Tools:      make(map[string]*Tool),
	}
}

func NewGoTool(module, version string) *Tool {
	return &Tool{
		Source:  "go",
		Module:  module,
		Version: version,
	}
}

func NewBinaryTool(version string) *Tool {
	return &Tool{
		Source:  "binary",
		Version: version,
	}
}

func NewBuiltTool(path string) *Tool {
	return &Tool{
		Source: "built",
		Path:   path,
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

		switch tool.Source {
		case "go":
			if err := cmds.DownloadGoTool(name, tool.Module, tool.Version, goos, goarch, binPath); err != nil {
				return fmt.Errorf("failed to download %s: %w", name, err)
			}

		case "binary":
			if name == "tailwindcli" {
				if err := cmds.DownloadTailwindCLI(tool.Version, goos, goarch, binPath); err != nil {
					return fmt.Errorf("failed to download %s: %w", name, err)
				}
			} else {
				return fmt.Errorf("unknown binary tool: %s", name)
			}

		case "built":
			if name == "run" {
				if err := cmds.RunGoRunBin(absTargetDir); err != nil {
					return fmt.Errorf("failed to build %s: %w", name, err)
				}
			} else {
				return fmt.Errorf("unknown built binary: %s", name)
			}
		}
	}

	if err := l.WriteLockFile(absTargetDir); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	return nil
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

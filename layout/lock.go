package layout

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type AndurelLock struct {
	Version    string                `json:"version"`
	Extensions map[string]*Extension `json:"extensions,omitempty"`
	Tools      map[string]*Tool      `json:"tools"`
}

type Extension struct {
	AppliedAt string `json:"appliedAt"`
}

type Tool struct {
	Source   string `json:"source"`
	Version  string `json:"version"`
	Module   string `json:"module,omitempty"`
	Checksum string `json:"checksum,omitempty"`
	Path     string `json:"path,omitempty"`
}

func NewAndurelLock(version string) *AndurelLock {
	return &AndurelLock{
		Version:    version,
		Extensions: make(map[string]*Extension),
		Tools:      make(map[string]*Tool),
	}
}

func NewGoTool(module, version, checksum string) *Tool {
	return &Tool{
		Source:   "go",
		Module:   module,
		Version:  version,
		Checksum: checksum,
	}
}

func NewBinaryTool(version, checksum string) *Tool {
	return &Tool{
		Source:   "binary",
		Version:  version,
		Checksum: checksum,
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
	lockPath := filepath.Join(targetDir, "andurel.lock")

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}

	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

func ReadLockFile(targetDir string) (*AndurelLock, error) {
	lockPath := filepath.Join(targetDir, "andurel.lock")

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

func ValidateBinaryChecksum(binaryPath, expectedChecksum string) error {
	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to open binary: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := fmt.Sprintf("sha256:%x", h.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

func CalculateBinaryChecksum(binaryPath string) (string, error) {
	f, err := os.Open(binaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open binary: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}


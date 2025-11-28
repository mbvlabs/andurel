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
	Version  string              `json:"version"`
	Binaries map[string]*Binary  `json:"binaries"`
}

type Binary struct {
	Version  string `json:"version,omitempty"`
	URL      string `json:"url,omitempty"`
	Checksum string `json:"checksum,omitempty"`
	Type     string `json:"type,omitempty"`
	Source   string `json:"source,omitempty"`
}

func NewAndurelLock() *AndurelLock {
	return &AndurelLock{
		Version:  "1",
		Binaries: make(map[string]*Binary),
	}
}

func (l *AndurelLock) AddBinary(name, version, url, checksum string) {
	l.Binaries[name] = &Binary{
		Version:  version,
		URL:      url,
		Checksum: checksum,
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


// Package files provides utilities for file and directory management.
package files

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) EnsureDirectoryExists(dirPath string) error {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}
	return nil
}

func (m *Manager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (m *Manager) ValidateFileNotExists(path string) error {
	if m.FileExists(path) {
		return fmt.Errorf("file %s already exists", path)
	}
	return nil
}

func (m *Manager) WriteFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := m.EnsureDirectoryExists(dir); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0600)
}

func (m *Manager) RunSQLCGenerate() error {
	cmd := exec.CommandContext(
		context.Background(),
		"just",
		"generate-db-functions",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to run 'just generate-db-functions': %w\nOutput: %s",
			err,
			output,
		)
	}
	fmt.Println("Generated database functions with sqlc")
	return nil
}

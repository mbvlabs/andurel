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
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
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

	return os.WriteFile(path, []byte(content), 0o600)
}

func (m *Manager) RunSQLCGenerate() error {
	// Find the root directory with go.mod to run from project root
	rootDir, err := m.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

	// Run sqlc compile
	compileCmd := exec.CommandContext(
		context.Background(),
		"go", "tool", "sqlc", "-f", "./database/sqlc.yaml", "compile",
	)
	compileCmd.Dir = rootDir
	if output, err := compileCmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"failed to run 'go tool sqlc compile': %w\nOutput: %s",
			err,
			output,
		)
	}

	// Run sqlc generate
	generateCmd := exec.CommandContext(
		context.Background(),
		"go", "tool", "sqlc", "-f", "./database/sqlc.yaml", "generate",
	)
	generateCmd.Dir = rootDir
	if output, err := generateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"failed to run 'go tool sqlc generate': %w\nOutput: %s",
			err,
			output,
		)
	}

	fmt.Println("Generated database functions with sqlc")
	return nil
}

func (m *Manager) FindGoModRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found")
}

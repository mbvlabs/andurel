package files

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/mbvlabs/andurel/pkg/constants"
)

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) EnsureDirectoryExists(dirPath string) error {
	if err := os.MkdirAll(dirPath, constants.DirPermissionDefault); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}
	return nil
}

func (m *Manager) FileExists(path string) bool {
	return cache.GetFileExists("file_exists:"+path, func() bool {
		_, err := os.Stat(path)
		return err == nil
	})
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

	return os.WriteFile(path, []byte(content), constants.FilePermissionPrivate)
}

func (m *Manager) RunSQLCGenerate() error {
	rootDir, err := m.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}

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
	return cache.GetDirectoryRoot("go_mod_root", func() (string, error) {
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
	})
}

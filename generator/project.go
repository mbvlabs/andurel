package generator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/cache"
)

type ProjectManager struct {
	modulePath  string
	fileManager files.Manager
}

func NewProjectManager() (*ProjectManager, error) {
	fm := files.NewUnifiedFileManager()
	modulePath, err := getCurrentModulePath(fm)
	if err != nil {
		return nil, fmt.Errorf("failed to get module path: %w", err)
	}

	return &ProjectManager{
		modulePath:  modulePath,
		fileManager: fm,
	}, nil
}

func (pm *ProjectManager) GetModulePath() string {
	return pm.modulePath
}

func getCurrentModulePath(fileManager files.Manager) (string, error) {
	return cache.GetModulePath("current_module_path", func() (string, error) {
		rootDir, err := fileManager.FindGoModRoot()
		if err != nil {
			return "", fmt.Errorf("failed to find go.mod: %w", err)
		}

		goModPath := filepath.Join(rootDir, "go.mod")
		file, err := os.Open(goModPath)
		if err != nil {
			return "", fmt.Errorf("failed to open go.mod: %w", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "module ") {
				return strings.Fields(line)[1], nil
			}
		}

		return "", fmt.Errorf("module declaration not found in go.mod")
	})
}

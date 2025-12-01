package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mbvlabs/andurel/layout"
)

type TemplateGenerator struct {
	targetVersion string
}

func NewTemplateGenerator(targetVersion string) *TemplateGenerator {
	return &TemplateGenerator{
		targetVersion: targetVersion,
	}
}

func (g *TemplateGenerator) Generate(config layout.ScaffoldConfig, projectRoot string) (shadowDir string, err error) {
	timestamp := time.Now().Format("20060102-150405")
	shadowDirName := fmt.Sprintf(".andurel-upgrade-%s", timestamp)

	parentDir := filepath.Dir(projectRoot)
	shadowPath := filepath.Join(parentDir, shadowDirName)

	if err := os.MkdirAll(shadowPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create shadow directory: %w", err)
	}

	if err := layout.Scaffold(
		shadowPath,
		config.ProjectName,
		config.Repository,
		config.Database,
		config.CSSFramework,
		g.targetVersion,
		config.Extensions,
	); err != nil {
		g.Cleanup(shadowPath)
		return "", fmt.Errorf("failed to scaffold shadow project: %w", err)
	}

	return shadowPath, nil
}

func (g *TemplateGenerator) Cleanup(shadowDir string) error {
	if shadowDir == "" {
		return fmt.Errorf("shadow directory path is empty")
	}

	if err := os.RemoveAll(shadowDir); err != nil {
		return fmt.Errorf("failed to remove shadow directory: %w", err)
	}

	return nil
}

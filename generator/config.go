package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/generator/files"
	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Database  DatabaseConfig
	Files     FileConfig
	Templates TemplateConfig
	Paths     PathConfig
}

type DatabaseConfig struct {
	Type          string
	MigrationDirs []string
	DefaultSchema string
}

type FileConfig struct {
	PrivatePermission os.FileMode
	DirPermission     os.FileMode
}

type TemplateConfig struct {
	CacheEnabled bool
}

type PathConfig struct {
	Models      string
	Controllers string
	Views       string
	Routes      string
	Queries     string
}

type BobGenConfig struct {
	Sqlite Sqlite `yaml:"sqlite,omitempty"`
	PSQL   PSQL   `yaml:"psql,omitempty"`
}

type Sqlite struct {
	DSN string `yaml:"dsn"`
}

type PSQL struct {
	DSN string `yaml:"dsn"`
}

func readDatabaseTypeFromBobYaml() (string, error) {
	manager := files.NewManager()
	rootDir, err := manager.FindGoModRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find go.mod root: %w", err)
	}

	bobGenPath := filepath.Join(rootDir, "database", "bobgen.yaml")

	if _, err := os.Stat(bobGenPath); os.IsNotExist(err) {
		return "", fmt.Errorf("sqlc.yaml not found at %s", bobGenPath)
	}

	data, err := os.ReadFile(bobGenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read sqlc.yaml: %w", err)
	}

	var config BobGenConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse sqlc.yaml: %w", err)
	}

	if config.PSQL.DSN == "" && config.Sqlite.DSN != "" {
		return "sqlite", nil
	}

	return "postgresql", nil
}

func NewDefaultAppConfig() *AppConfig {
	databaseType := "postgresql" // fallback default
	if dbType, err := readDatabaseTypeFromBobYaml(); err == nil {
		databaseType = dbType
	}

	return &AppConfig{
		Database: DatabaseConfig{
			Type:          databaseType,
			MigrationDirs: []string{"database/migrations"},
			DefaultSchema: "public",
		},
		Files: FileConfig{
			PrivatePermission: 0o600,
			DirPermission:     0o755,
		},
		Templates: TemplateConfig{
			CacheEnabled: true,
		},
		Paths: PathConfig{
			Models:      "models",
			Controllers: "controllers",
			Views:       "views",
			Routes:      "router/routes",
			Queries:     "database/queries",
		},
	}
}

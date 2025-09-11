package generator

import (
	"fmt"
	"os"

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

// SQLCConfig represents the structure of sqlc.yaml file
type SQLCConfig struct {
	SQL []SQLConfig `yaml:"sql"`
}

type SQLConfig struct {
	Engine string `yaml:"engine"`
}

// readDatabaseTypeFromSQLCYAML reads the database type from database/sqlc.yaml
func readDatabaseTypeFromSQLCYAML() (string, error) {
	sqlcPath := "database/sqlc.yaml"
	
	// Check if sqlc.yaml exists
	if _, err := os.Stat(sqlcPath); os.IsNotExist(err) {
		return "", fmt.Errorf("sqlc.yaml not found at %s", sqlcPath)
	}
	
	// Read the file
	data, err := os.ReadFile(sqlcPath)
	if err != nil {
		return "", fmt.Errorf("failed to read sqlc.yaml: %w", err)
	}
	
	// Parse YAML
	var config SQLCConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse sqlc.yaml: %w", err)
	}
	
	// Extract engine from first SQL config
	if len(config.SQL) == 0 {
		return "", fmt.Errorf("no SQL configuration found in sqlc.yaml")
	}
	
	engine := config.SQL[0].Engine
	if engine != "postgresql" && engine != "sqlite" {
		return "", fmt.Errorf("unsupported database engine: %s (supported: postgresql, sqlite)", engine)
	}
	
	return engine, nil
}

func NewDefaultAppConfig() *AppConfig {
	// Try to read database type from sqlc.yaml first
	databaseType := "postgresql" // fallback default
	if dbType, err := readDatabaseTypeFromSQLCYAML(); err == nil {
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

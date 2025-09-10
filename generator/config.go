package generator

import (
	"os"
)

type AppConfig struct {
	Database  DatabaseConfig
	Files     FileConfig
	Templates TemplateConfig
	Paths     PathConfig
}

type DatabaseConfig struct {
	Type           string
	MigrationDirs  []string
	DefaultSchema  string
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

func NewDefaultAppConfig() *AppConfig {
	return &AppConfig{
		Database: DatabaseConfig{
			Type:          "postgresql",
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
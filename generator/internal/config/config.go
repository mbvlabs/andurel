package config

// Config configures the value.
type Config struct {
	MigrationDirs []string
	DatabaseType  string
	TableName     string
	OutputFile    string
	PackageName   string
	GenerateJSON  bool
}

// NewDefaultConfig creates a new default config.
func NewDefaultConfig() *Config {
	return &Config{
		MigrationDirs: []string{"database/migrations"},
		DatabaseType:  "postgresql",
		PackageName:   "models",
		GenerateJSON:  true,
	}
}

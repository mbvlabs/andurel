package config

type Config struct {
	MigrationDirs []string
	DatabaseType  string
	TableName     string
	OutputFile    string
	PackageName   string
	GenerateJSON  bool
}

func NewDefaultConfig() *Config {
	return &Config{
		MigrationDirs: []string{"database/migrations"},
		DatabaseType:  "postgresql",
		PackageName:   "models",
		GenerateJSON:  true,
	}
}

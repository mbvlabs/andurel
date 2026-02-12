package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/generator/files"
	"gopkg.in/yaml.v3"
)

// UnifiedConfig represents the single source of truth for all configuration
type UnifiedConfig struct {
	Database   DatabaseConfig   `yaml:"database"`
	Paths      PathConfig       `yaml:"paths"`
	Templates  TemplateConfig   `yaml:"templates"`
	Files      FileConfig       `yaml:"files"`
	Project    ProjectConfig    `yaml:"project"`
	Generation GenerationConfig `yaml:"generation"`
}

// DatabaseConfig contains database-specific configuration
type DatabaseConfig struct {
	Type          string   `yaml:"type"`
	MigrationDirs []string `yaml:"migration_dirs"`
	DefaultSchema string   `yaml:"default_schema"`
	Driver        string   `yaml:"driver"`
	Method        string   `yaml:"method"`
}

// PathConfig contains path configurations
type PathConfig struct {
	Models      string `yaml:"models"`
	Controllers string `yaml:"controllers"`
	Views       string `yaml:"views"`
	Routes      string `yaml:"routes"`
	Queries     string `yaml:"queries"`
	Migrations  string `yaml:"migrations"`
	Database    string `yaml:"database"`
}

// TemplateConfig contains template-related configuration
type TemplateConfig struct {
	CacheEnabled bool `yaml:"cache_enabled"`
	CacheTTL     int  `yaml:"cache_ttl"`
}

// FileConfig contains file-related configuration
type FileConfig struct {
	PrivatePermission os.FileMode `yaml:"private_permission"`
	DirPermission     os.FileMode `yaml:"dir_permission"`
}

// ProjectConfig contains project-specific configuration
type ProjectConfig struct {
	Name        string `yaml:"name"`
	ModulePath  string `yaml:"module_path"`
	PackageName string `yaml:"package_name"`
}

// GenerationConfig contains code generation configuration
type GenerationConfig struct {
	GenerateJSON bool   `yaml:"generate_json"`
	OutputFormat string `yaml:"output_format"`
}

// ConfigManager manages configuration loading and validation
type ConfigManager struct {
	config    *UnifiedConfig
	validator *ConfigValidator
}

// ConfigValidator validates configuration values
type ConfigValidator struct{}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		validator: &ConfigValidator{},
	}
}

// Load loads configuration from defaults
func (cm *ConfigManager) Load() (*UnifiedConfig, error) {
	databaseType := "postgresql"

	config := &UnifiedConfig{
		Database: DatabaseConfig{
			Type:          databaseType,
			MigrationDirs: []string{"database/migrations"},
			DefaultSchema: "public",
			Driver:        cm.getDatabaseDriver(databaseType),
			Method:        "Conn",
		},
		Paths: PathConfig{
			Models:      "models",
			Controllers: "controllers",
			Views:       "views",
			Routes:      "router/routes",
			Queries:     "database/queries",
			Migrations:  "database/migrations",
			Database:    "database",
		},
		Templates: TemplateConfig{
			CacheEnabled: true,
			CacheTTL:     3600, // 1 hour
		},
		Files: FileConfig{
			PrivatePermission: 0o600,
			DirPermission:     0o755,
		},
		Project: ProjectConfig{
			PackageName: "models",
		},
		Generation: GenerationConfig{
			GenerateJSON: true,
			OutputFormat: "go",
		},
	}

	// Validate configuration
	if err := cm.validator.Validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	cm.config = config
	return config, nil
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *UnifiedConfig {
	if cm.config == nil {
		config, _ := cm.Load()
		return config
	}
	return cm.config
}

// getDatabaseDriver returns the appropriate driver for the database type
func (cm *ConfigManager) getDatabaseDriver(dbType string) string {
	return "pgx"
}

// Validate validates the configuration
func (cv *ConfigValidator) Validate(config *UnifiedConfig) error {
	// Validate database type
	if config.Database.Type != "postgresql" {
		return fmt.Errorf(
			"unsupported database type: %s (only postgresql is supported)",
			config.Database.Type,
		)
	}

	// Validate required paths
	if config.Paths.Models == "" {
		return fmt.Errorf("models path cannot be empty")
	}
	if config.Paths.Controllers == "" {
		return fmt.Errorf("controllers path cannot be empty")
	}
	if config.Paths.Views == "" {
		return fmt.Errorf("views path cannot be empty")
	}

	// Validate file permissions
	if config.Files.PrivatePermission == 0 {
		config.Files.PrivatePermission = 0o600
	}
	if config.Files.DirPermission == 0 {
		config.Files.DirPermission = 0o755
	}

	return nil
}

// GetModelConfig returns configuration for model generation
func (uc *UnifiedConfig) GetModelConfig() ModelConfig {
	return ModelConfig{
		TableName:    "", // Set per generation
		ResourceName: "", // Set per generation
		PackageName:  uc.Project.PackageName,
		DatabaseType: uc.Database.Type,
		ModulePath:   uc.Project.ModulePath,
		Paths: ModelPaths{
			Models:  uc.Paths.Models,
			Queries: uc.Paths.Queries,
		},
		Generation: uc.Generation,
	}
}

// GetControllerConfig returns configuration for controller generation
func (uc *UnifiedConfig) GetControllerConfig() ControllerConfig {
	return ControllerConfig{
		ResourceName: "", // Set per generation
		PluralName:   "", // Set per generation
		PackageName:  uc.Project.PackageName,
		ModulePath:   uc.Project.ModulePath,
		Paths: ControllerPaths{
			Controllers: uc.Paths.Controllers,
			Routes:      uc.Paths.Routes,
		},
	}
}

// GetViewConfig returns configuration for view generation
func (uc *UnifiedConfig) GetViewConfig() ViewConfig {
	return ViewConfig{
		ResourceName: "", // Set per generation
		PluralName:   "", // Set per generation
		ModulePath:   uc.Project.ModulePath,
		Paths: ViewPaths{
			Views: uc.Paths.Views,
		},
	}
}

type ModelConfig struct {
	TableName    string           `json:"table_name"`
	ResourceName string           `json:"resource_name"`
	PackageName  string           `json:"package_name"`
	DatabaseType string           `json:"database_type"`
	ModulePath   string           `json:"module_path"`
	Paths        ModelPaths       `json:"paths"`
	Generation   GenerationConfig `json:"generation"`
}

type ModelPaths struct {
	Models  string `json:"models"`
	Queries string `json:"queries"`
}

type ControllerConfig struct {
	ResourceName string          `json:"resource_name"`
	PluralName   string          `json:"plural_name"`
	PackageName  string          `json:"package_name"`
	ModulePath   string          `json:"module_path"`
	Paths        ControllerPaths `json:"paths"`
}

type ControllerPaths struct {
	Controllers string `json:"controllers"`
	Routes      string `json:"routes"`
}

type ViewConfig struct {
	ResourceName string    `json:"resource_name"`
	PluralName   string    `json:"plural_name"`
	ModulePath   string    `json:"module_path"`
	Paths        ViewPaths `json:"paths"`
}

type ViewPaths struct {
	Views string `json:"views"`
}

// Global config manager instance
var globalConfigManager *ConfigManager

// GetGlobalConfigManager returns the global configuration manager
func GetGlobalConfigManager() *ConfigManager {
	if globalConfigManager == nil {
		globalConfigManager = NewConfigManager()
	}
	return globalConfigManager
}

// GetGlobalConfig returns the global configuration
func GetGlobalConfig() *UnifiedConfig {
	return GetGlobalConfigManager().GetConfig()
}

// readDatabaseTypeFromSQLCYAML reads database type from sqlc.yaml
func readDatabaseTypeFromSQLCYAML() (string, error) {
	manager := files.NewUnifiedFileManager()
	rootDir, err := manager.FindGoModRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find go.mod root: %w", err)
	}

	sqlcPath := filepath.Join(rootDir, "internal", "storage", "andurel_sqlc_config.yaml")
	if _, err := os.Stat(sqlcPath); err != nil {
		if os.IsNotExist(err) {
			return "postgresql", nil
		}
		return "", fmt.Errorf("failed to read sqlc configuration: %w", err)
	}

	data, err := os.ReadFile(sqlcPath)
	if err != nil {
		return "", fmt.Errorf("failed to read sqlc.yaml: %w", err)
	}

	var config SQLCConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse sqlc.yaml: %w", err)
	}

	if len(config.SQL) == 0 {
		return "", fmt.Errorf("no SQL configuration found in sqlc.yaml")
	}

	engine := config.SQL[0].Engine
	if engine != "postgresql" {
		return "", fmt.Errorf(
			"unsupported database engine: %s (only postgresql is supported)",
			engine,
		)
	}

	return engine, nil
}

// SQLCConfig for reading sqlc.yaml files
type SQLCConfig struct {
	SQL []SQLConfig `yaml:"sql"`
}

type SQLConfig struct {
	Engine string `yaml:"engine"`
}

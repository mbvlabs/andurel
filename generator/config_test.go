package generator

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestNewDefaultAppConfig(t *testing.T) {
	cache.ClearFileSystemCache()

	configManager := NewConfigManager()
	config, err := configManager.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Database.Type != "postgresql" {
		t.Errorf("Expected postgresql, got %s", config.Database.Type)
	}
	if config.Database.Driver != "pgx" {
		t.Errorf("Expected pgx driver, got %s", config.Database.Driver)
	}
	if config.Paths.Models != "models" {
		t.Errorf("Expected models path, got %s", config.Paths.Models)
	}
}

func TestConfigManagerGetConfigLoadsOnce(t *testing.T) {
	cm := NewConfigManager()

	config := cm.GetConfig()
	if config == nil {
		t.Fatal("GetConfig returned nil")
	}
	config.Project.ModulePath = "example.com/app"

	if got := cm.GetConfig(); got.Project.ModulePath != "example.com/app" {
		t.Fatalf("GetConfig should return cached config, got module path %q", got.Project.ModulePath)
	}
}

func TestConfigValidatorValidate(t *testing.T) {
	validator := &ConfigValidator{}

	config := &UnifiedConfig{
		Database: DatabaseConfig{Type: "postgresql"},
		Paths: PathConfig{
			Models:      "models",
			Controllers: "controllers",
			Views:       "views",
		},
	}
	if err := validator.Validate(config); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if config.Files.PrivatePermission != 0o600 {
		t.Fatalf("PrivatePermission = %v", config.Files.PrivatePermission)
	}
	if config.Files.DirPermission != 0o755 {
		t.Fatalf("DirPermission = %v", config.Files.DirPermission)
	}

	tests := []struct {
		name   string
		config *UnifiedConfig
		want   string
	}{
		{
			name: "unsupported database",
			config: &UnifiedConfig{
				Database: DatabaseConfig{Type: "mysql"},
				Paths:    PathConfig{Models: "models", Controllers: "controllers", Views: "views"},
			},
			want: "unsupported database type",
		},
		{
			name:   "missing models",
			config: &UnifiedConfig{Database: DatabaseConfig{Type: "postgresql"}, Paths: PathConfig{Controllers: "controllers", Views: "views"}},
			want:   "models path cannot be empty",
		},
		{
			name:   "missing controllers",
			config: &UnifiedConfig{Database: DatabaseConfig{Type: "postgresql"}, Paths: PathConfig{Models: "models", Views: "views"}},
			want:   "controllers path cannot be empty",
		},
		{
			name:   "missing views",
			config: &UnifiedConfig{Database: DatabaseConfig{Type: "postgresql"}, Paths: PathConfig{Models: "models", Controllers: "controllers"}},
			want:   "views path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.config)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestUnifiedConfigGenerationConfigs(t *testing.T) {
	config := &UnifiedConfig{
		Database: DatabaseConfig{Type: "postgresql"},
		Paths: PathConfig{
			Models:      "app/models",
			Controllers: "app/controllers",
			Routes:      "router/routes",
			Views:       "app/views",
		},
		Project: ProjectConfig{
			PackageName: "models",
			ModulePath:  "example.com/app",
		},
		Generation: GenerationConfig{
			GenerateJSON: true,
			OutputFormat: "go",
		},
	}

	model := config.GetModelConfig()
	if model.PackageName != "models" || model.DatabaseType != "postgresql" || model.Paths.Models != "app/models" {
		t.Fatalf("unexpected model config: %#v", model)
	}
	if !model.Generation.GenerateJSON || model.Generation.OutputFormat != "go" {
		t.Fatalf("unexpected generation config: %#v", model.Generation)
	}

	controller := config.GetControllerConfig()
	if controller.PackageName != "models" || controller.ModulePath != "example.com/app" {
		t.Fatalf("unexpected controller config: %#v", controller)
	}
	if controller.Paths.Controllers != "app/controllers" || controller.Paths.Routes != "router/routes" {
		t.Fatalf("unexpected controller paths: %#v", controller.Paths)
	}

	view := config.GetViewConfig()
	if view.ModulePath != "example.com/app" || view.Paths.Views != "app/views" {
		t.Fatalf("unexpected view config: %#v", view)
	}
}

func TestGlobalConfigHelpers(t *testing.T) {
	old := globalConfigManager
	t.Cleanup(func() {
		globalConfigManager = old
	})
	globalConfigManager = nil

	manager := GetGlobalConfigManager()
	if manager == nil {
		t.Fatal("GetGlobalConfigManager returned nil")
	}
	if GetGlobalConfigManager() != manager {
		t.Fatal("GetGlobalConfigManager should return singleton")
	}
	if config := GetGlobalConfig(); config == nil || config.Database.Type != "postgresql" {
		t.Fatalf("GetGlobalConfig returned %#v", config)
	}
}

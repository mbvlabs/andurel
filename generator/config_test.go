package generator

import (
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

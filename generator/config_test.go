package generator

import (
	"os"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestReadDatabaseTypeFromSQLCYAML(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
		hasError bool
	}{
		{
			name: "PostgreSQL engine",
			content: `version: "2"
sql:
  - engine: postgresql
    schema: migrations`,
			expected: "postgresql",
			hasError: false,
		},
		{
			name: "Unsupported engine",
			content: `version: "2"
sql:
  - engine: mysql
    schema: migrations`,
			expected: "",
			hasError: true,
		},
		{
			name: "Empty user SQL list falls back to base config",
			content: `version: "2"
sql: []`,
			expected: "postgresql",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearFileSystemCache()

			tmpDir := t.TempDir()

			goModContent := "module test\n\ngo 1.21\n"
			if err := os.WriteFile(tmpDir+"/go.mod", []byte(goModContent), 0o644); err != nil {
				t.Fatalf("Failed to write go.mod: %v", err)
			}

			dbDir := tmpDir + "/database"
			if err := os.MkdirAll(dbDir, 0o755); err != nil {
				t.Fatalf("Failed to create database directory: %v", err)
			}

			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}

			sqlcPath := "database/sqlc.yaml"
			if err := os.WriteFile(sqlcPath, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("Failed to write sqlc.yaml: %v", err)
			}

			result, err := readDatabaseTypeFromSQLCYAML()

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestNewDefaultAppConfig_WithSQLCYAML(t *testing.T) {
	tests := []struct {
		name        string
		sqlcContent string
		expected    string
	}{
		{
			name: "Uses PostgreSQL from sqlc.yaml",
			sqlcContent: `version: "2"
sql:
  - engine: postgresql
    schema: migrations`,
			expected: "postgresql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearFileSystemCache()

			tmpDir := t.TempDir()

			goModContent := "module test\n\ngo 1.21\n"
			if err := os.WriteFile(tmpDir+"/go.mod", []byte(goModContent), 0o644); err != nil {
				t.Fatalf("Failed to write go.mod: %v", err)
			}

			dbDir := tmpDir + "/database"
			if err := os.MkdirAll(dbDir, 0o755); err != nil {
				t.Fatalf("Failed to create database directory: %v", err)
			}

			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}

			sqlcPath := "database/sqlc.yaml"
			if err := os.WriteFile(sqlcPath, []byte(tt.sqlcContent), 0o644); err != nil {
				t.Fatalf("Failed to write sqlc.yaml: %v", err)
			}

			configManager := NewConfigManager()
			config, err := configManager.Load()
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			if config.Database.Type != tt.expected {
				t.Errorf("Expected database type %s, got %s", tt.expected, config.Database.Type)
			}
		})
	}
}

func TestNewDefaultAppConfig_FallbackWhenNoSQLCYAML(t *testing.T) {
	cache.ClearFileSystemCache()

	tmpDir := t.TempDir()

	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(tmpDir+"/go.mod", []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	configManager := NewConfigManager()
	config, err := configManager.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Database.Type != "postgresql" {
		t.Errorf("Expected fallback to postgresql, got %s", config.Database.Type)
	}
}

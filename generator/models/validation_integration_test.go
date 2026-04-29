package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/generator/templates"
)

func TestModelFileGeneration_ValidPrimaryKeyTypes(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
		databaseType  string
	}{
		{
			name:          "Should accept PostgreSQL migration with TEXT primary key",
			migrationsDir: "text_pk",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			databaseType:  "postgresql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalWd, _ := os.Getwd()

			generator := NewGenerator(tt.databaseType)

			migrationsPath := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)
			cat, err := generator.BuildCatalogFromMigrations(tt.tableName, []string{migrationsPath})
			if err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}

			model, err := generator.Build(cat, Config{
				TableName:    tt.tableName,
				ResourceName: tt.resourceName,
				PackageName:  "models",
				DatabaseType: tt.databaseType,
				ModulePath:   tt.modulePath,
			})
			if err != nil {
				t.Fatalf("Failed to build model: %v", err)
			}

			// Generate model file
			templateContent, err := templates.Files.ReadFile("model.tmpl")
			if err != nil {
				t.Fatalf("Failed to read model template: %v", err)
			}

			modelContent, err := generator.GenerateModelFile(model, string(templateContent))
			if err != nil {
				t.Fatalf("Failed to generate model file: %v", err)
			}

			// Verify bun patterns
			if !containsAny(modelContent, "bun.BaseModel", `bun:"`) {
				t.Error("Generated code should contain bun tags and BaseModel")
			}
		})
	}
}

func TestBunModelGeneration_ValidatesIDColumns(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
		databaseType  string
		shouldFail    bool
	}{
		{
			name:          "Should succeed with valid PostgreSQL UUID primary key",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "postgresql",
			shouldFail:    false,
		},
		{
			name:          "Should succeed with valid PostgreSQL TEXT primary key",
			migrationsDir: "text_pk",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "postgresql",
			shouldFail:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalWd, _ := os.Getwd()

			generator := NewGenerator(tt.databaseType)

			migrationsPath := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)
			cat, err := generator.BuildCatalogFromMigrations(tt.tableName, []string{migrationsPath})
			if err != nil {
				if !tt.shouldFail {
					t.Fatalf("Failed to build catalog from migrations: %v", err)
				}
				return
			}

			if tt.shouldFail {
				t.Fatal("Expected error but got none")
			}

			model, err := generator.Build(cat, Config{
				TableName:    tt.tableName,
				ResourceName: tt.resourceName,
				PackageName:  "models",
				DatabaseType: tt.databaseType,
				ModulePath:   "github.com/example/test",
			})
			if err != nil {
				t.Fatalf("Failed to build model: %v", err)
			}

			// Verify ID field type
			for _, field := range model.Fields {
				if field.Name == "ID" {
					if field.IsPrimaryKey {
						if tt.databaseType == "postgresql" && field.Type != "uuid.UUID" && field.Type != "string" {
							t.Errorf("ID field should be uuid.UUID or string, got %s", field.Type)
						}
					}
				}
			}
		})
	}
}

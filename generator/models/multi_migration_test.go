package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/pkg/constants"
)

// TestMultiMigrationHandling verifies that the generator correctly handles
// tables that are defined and modified across multiple migration files
func TestMultiMigrationHandling(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
		databaseType  string
		expectError   bool
	}{
		{
			name:          "Should handle multi-migration for posts table",
			migrationsDir: "posts_multi_migration",
			tableName:     "posts",
			resourceName:  "Post",
			databaseType:  "postgresql",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			queriesDir := filepath.Join(tempDir, "database", "queries")
			modelsDir := filepath.Join(tempDir, "models")

			err := os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			err = os.MkdirAll(modelsDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create models directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			generator := NewGenerator(tt.databaseType)

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}

			// Verify the catalog was built successfully
			table, err := cat.GetTable("", tt.tableName)
			if err != nil {
				t.Fatalf("Failed to get table from catalog: %v", err)
			}

			// Verify table has columns (basic sanity check)
			if len(table.Columns) == 0 {
				t.Error("Expected table to have columns from multi-migration files")
			}

			// Build model to ensure it works
			model, err := generator.Build(cat, Config{
				TableName:    tt.tableName,
				ResourceName: tt.resourceName,
				PackageName:  "models",
				DatabaseType: tt.databaseType,
				ModulePath:   "github.com/example/test",
			})
			if err != nil {
				t.Fatalf("Failed to build model from multi-migration catalog: %v", err)
			}

			// Verify model has fields
			if len(model.Fields) == 0 {
				t.Error("Expected model to have fields from multi-migration files")
			}
		})
	}
}

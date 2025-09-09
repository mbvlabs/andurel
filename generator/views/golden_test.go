package views

import (
	"flag"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerator_GoldenFiles(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
	}{
		{
			name:          "resource_user_view",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
		},
		{
			name:          "resource_product_view",
			migrationsDir: "product_table_with_decimals",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/shop",
		},
		{
			name:          "multi_migration_posts_view",
			migrationsDir: "posts_multi_migration",
			tableName:     "posts",
			resourceName:  "Post",
			modulePath:    "github.com/example/blog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			viewsDir := filepath.Join(tempDir, "views")

			err := os.MkdirAll(viewsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create views directory: %v", err)
			}

			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			generator := NewGenerator("postgresql")

			// Build catalog from migrations
			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			allMigrations, err := migrations.DiscoverMigrations([]string{migrationsDir})
			if err != nil {
				t.Fatalf("Failed to discover migrations: %v", err)
			}

			cat := catalog.NewCatalog("public")
			for _, migration := range allMigrations {
				for _, statement := range migration.Statements {
					if err := ddl.ApplyDDL(cat, statement, migration.FilePath); err != nil {
						t.Fatalf("Failed to apply DDL from %s: %v", migration.FilePath, err)
					}
				}
			}

			// Build view from catalog
			view, err := generator.Build(cat, Config{
				ResourceName: tt.resourceName,
				PluralName:   tt.tableName,
				ModulePath:   tt.modulePath,
			})
			if err != nil {
				t.Fatalf("Failed to build view: %v", err)
			}

			// Generate view content
			viewContent, err := generator.GenerateViewFile(view)
			if err != nil {
				t.Fatalf("Failed to generate view content: %v", err)
			}

			// Write content to test file
			pluralName := tt.tableName
			viewPath := filepath.Join("views", pluralName+"_resource.templ")
			err = os.WriteFile(viewPath, []byte(viewContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write view file: %v", err)
			}

			t.Run("view_file", func(t *testing.T) {
				compareWithGolden(t, tt.name+"_resource.templ", viewPath, *update, originalWd)
			})
		})
	}
}

func compareWithGolden(
	t *testing.T,
	goldenFile, actualFile string,
	update bool,
	originalWd string,
) {
	actualContent, err := os.ReadFile(actualFile)
	if err != nil {
		t.Fatalf("Failed to read actual file %s: %v", actualFile, err)
	}

	goldenPath := filepath.Join(originalWd, "testdata", goldenFile)

	if update {
		err = os.MkdirAll(filepath.Dir(goldenPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}

		err = os.WriteFile(goldenPath, actualContent, 0644)
		if err != nil {
			t.Fatalf("Failed to update golden file %s: %v", goldenPath, err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expectedContent, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v (run with -update to create)", goldenPath, err)
	}

	actualStr := strings.TrimSpace(string(actualContent))
	expectedStr := strings.TrimSpace(string(expectedContent))

	if actualStr != expectedStr {
		t.Errorf("Generated content doesn't match golden file %s\n\nActual:\n%s\n\nExpected:\n%s",
			goldenPath, actualStr, expectedStr)
	}
}

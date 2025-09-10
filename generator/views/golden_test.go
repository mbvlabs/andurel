package views

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/sebdah/goldie/v2"
)

func TestViewFileGeneration__GoldenFile(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
	}{
		{
			name:          "Should generate user view",
			fileName:      "user_view_resource",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
		},
		{
			name:          "Should generate product view",
			fileName:      "product_view_resource",
			migrationsDir: "product_table_with_decimals",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/shop",
		},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			viewsDir := filepath.Join(tempDir, "views")

			err := os.MkdirAll(viewsDir, 0o755)
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
			err = os.WriteFile(viewPath, []byte(viewContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write view file: %v", err)
			}

			// Read generated content
			generatedViewContent, err := os.ReadFile(viewPath)
			if err != nil {
				t.Fatalf("Failed to read generated view file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".templ"))

			g.Assert(t, tt.fileName, generatedViewContent)
		})
	}
}

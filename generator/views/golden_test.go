package views

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/pkg/constants"
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
		withController bool
	}{
		{
			name:           "Should generate user view with controller",
			fileName:       "user_view_resource",
			migrationsDir:  "simple_user_table",
			tableName:      "users",
			resourceName:   "User",
			modulePath:     "github.com/example/myapp",
			withController: true,
		},
		{
			name:           "Should generate product view with controller",
			fileName:       "product_view_resource",
			migrationsDir:  "product_table_with_decimals",
			tableName:      "products",
			resourceName:   "Product",
			modulePath:     "github.com/example/shop",
			withController: true,
		},
		{
			name:           "Should generate user view without controller",
			fileName:       "user_view_resource_no_controller",
			migrationsDir:  "simple_user_table",
			tableName:      "users",
			resourceName:   "User",
			modulePath:     "github.com/example/myapp",
			withController: false,
		},
		{
			name:           "Should generate product view without controller",
			fileName:       "product_view_resource_no_controller",
			migrationsDir:  "product_table_with_decimals",
			tableName:      "products",
			resourceName:   "Product",
			modulePath:     "github.com/example/shop",
			withController: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			viewsDir := filepath.Join(tempDir, "views")

			err := os.MkdirAll(viewsDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create views directory: %v", err)
			}

			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			generator := NewGenerator("postgresql")

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

			view, err := generator.Build(cat, Config{
				ResourceName: tt.resourceName,
				PluralName:   tt.tableName,
				ModulePath:   tt.modulePath,
			})
			if err != nil {
				t.Fatalf("Failed to build view: %v", err)
			}

			viewContent, err := generator.GenerateViewFile(view, tt.withController)
			if err != nil {
				t.Fatalf("Failed to generate view content: %v", err)
			}

			pluralName := tt.tableName
			viewPath := filepath.Join("views", pluralName+"_resource.templ")
			err = os.WriteFile(viewPath, []byte(viewContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write view file: %v", err)
			}

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

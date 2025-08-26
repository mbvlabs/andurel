package controllers

import (
	"flag"
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/internal/ddl"
	"mbvlabs/andurel/generator/internal/migrations"
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
		controllerType ControllerType
	}{
		{
			name:          "resource_user_controller",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			controllerType: ResourceController,
		},
		{
			name:          "resource_product_controller",
			migrationsDir: "product_table_with_decimals",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/shop",
			controllerType: ResourceController,
		},
		{
			name:          "normal_dashboard_controller",
			migrationsDir: "", // Not needed for normal controllers
			tableName:     "",
			resourceName:  "Dashboard",
			modulePath:    "github.com/example/myapp",
			controllerType: NormalController,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			controllersDir := filepath.Join(tempDir, "controllers")
			routesDir := filepath.Join(tempDir, "router", "routes")

			err := os.MkdirAll(controllersDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create controllers directory: %v", err)
			}

			err = os.MkdirAll(routesDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create routes directory: %v", err)
			}

			// Create a basic routes.go file for testing  
			routesContent := `package routes

import "github.com/labstack/echo/v4"

type Route struct {
	Name         string
	Path         string
	Handler      string
	HandleMethod string
	Method       string
	Middleware   []func(next echo.HandlerFunc) echo.HandlerFunc
}

var BuildRoutes = func() []Route {
	var r []Route

	r = append(
		r,
		assetRoutes...,
	)

	r = append(
		r,
		pageRoutes...,
	)

	r = append(
		r,
		apiRoutes...,
	)

	return r
}()
`
			routesFile := filepath.Join(routesDir, "routes.go")
			err = os.WriteFile(routesFile, []byte(routesContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create routes.go: %v", err)
			}

			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			generator := NewGenerator("postgresql")
			
			var cat *catalog.Catalog
			
			if tt.controllerType == ResourceController && tt.migrationsDir != "" {
				// Build catalog from migrations for resource controllers
				migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)
				
				allMigrations, err := migrations.DiscoverMigrations([]string{migrationsDir})
				if err != nil {
					t.Fatalf("Failed to discover migrations: %v", err)
				}

				cat = catalog.NewCatalog("public")
				for _, migration := range allMigrations {
					for _, statement := range migration.Statements {
						if err := ddl.ApplyDDL(cat, statement, migration.FilePath); err != nil {
							t.Fatalf("Failed to apply DDL from %s: %v", migration.FilePath, err)
						}
					}
				}
			} else {
				// Empty catalog for normal controllers
				cat = catalog.NewCatalog("public")
			}

			err = generator.GenerateController(cat, tt.resourceName, tt.controllerType, tt.modulePath)
			if err != nil {
				t.Fatalf("Failed to generate controller: %v", err)
			}

			// Test controller file
			pluralName := strings.ToLower(tt.resourceName)
			if tt.controllerType == ResourceController {
				// Use the actual table name from test config
				if tt.tableName != "" {
					pluralName = tt.tableName
				} else {
					pluralName = strings.ToLower(tt.resourceName) + "s" // Simple pluralization for tests
				}
			} else {
				// For normal controllers, use simple pluralization
				pluralName = strings.ToLower(tt.resourceName) + "s"
			}
			controllerPath := filepath.Join("controllers", pluralName+".go")

			t.Run("controller_file", func(t *testing.T) {
				compareWithGolden(t, tt.name+"_controller.go", controllerPath, *update, originalWd)
			})

			// Test routes file (only for resource controllers)
			if tt.controllerType == ResourceController {
				routesPath := filepath.Join("router", "routes", pluralName+".go")
				t.Run("routes_file", func(t *testing.T) {
					compareWithGolden(t, tt.name+"_routes.go", routesPath, *update, originalWd)
				})

				// Test that routes.go was properly updated with the new routes
				t.Run("routes_registration", func(t *testing.T) {
					updatedRoutesPath := filepath.Join("router", "routes", "routes.go")
					compareWithGolden(t, tt.name+"_routes_registration.go", updatedRoutesPath, *update, originalWd)
				})
			}
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
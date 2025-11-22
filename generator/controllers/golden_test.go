package controllers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/sebdah/goldie/v2"
)

func TestControllerFileGeneration__GoldenFile(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
		ctrlType      ControllerType
	}{
		{
			name:          "Should generate user controller with views",
			fileName:      "user_controller",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			ctrlType:      ResourceController,
		},
		{
			name:          "Should generate product controller with views",
			fileName:      "product_controller",
			migrationsDir: "product_table",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/shop",
			ctrlType:      ResourceController,
		},
		{
			name:          "Should generate product controller with no views",
			fileName:      "product_controller_no_view",
			migrationsDir: "product_table",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/shop",
			ctrlType:      ResourceControllerNoViews,
		},
		{
			name:          "Should generate admin user controller with views",
			fileName:      "admin_user_controller",
			migrationsDir: "admin_user_table",
			tableName:     "admin_users",
			resourceName:  "AdminUser",
			modulePath:    "github.com/example/myapp",
			ctrlType:      ResourceController,
		},
		{
			name:          "Should generate new user controller with no views",
			fileName:      "new_user_controller_no_view",
			migrationsDir: "new_user_table",
			tableName:     "new_users",
			resourceName:  "NewUser",
			modulePath:    "github.com/example/myapp",
			ctrlType:      ResourceControllerNoViews,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			controllersDir := filepath.Join(tempDir, "controllers")

			err := os.MkdirAll(controllersDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create controllers directory: %v", err)
			}

			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			generator := NewGenerator("postgresql")

			cat, err := buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}

			controller, err := generator.Build(cat, Config{
				ResourceName:   tt.resourceName,
				PluralName:     tt.tableName,
				PackageName:    "controllers",
				ModulePath:     tt.modulePath,
				ControllerType: tt.ctrlType,
			})
			if err != nil {
				t.Fatalf("Failed to build controller: %v", err)
			}

			templateRenderer := NewTemplateRenderer()
			controllerContent, err := templateRenderer.RenderControllerFile(controller)
			if err != nil {
				t.Fatalf("Failed to render controller content: %v", err)
			}

			controllerPath := filepath.Join("controllers", tt.tableName+".go")

			err = os.WriteFile(controllerPath, []byte(controllerContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write controller file: %v", err)
			}

			routeGenerator := NewRouteGenerator()
			err = routeGenerator.formatGoFile(controllerPath)
			if err != nil {
				t.Fatalf("Failed to format controller file: %v", err)
			}

			formattedControllerContent, err := os.ReadFile(controllerPath)
			if err != nil {
				t.Fatalf("Failed to read formatted controller file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

			g.Assert(t, tt.fileName, formattedControllerContent)
		})
	}
}

func TestRoutesFileGeneration__GoldenFile(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		tableName    string
		resourceName string
	}{
		{
			name:         "Should generate routes for users controller",
			fileName:     "user_controller_routes",
			tableName:    "users",
			resourceName: "User",
		},
		{
			name:         "Should generate routes for products controller",
			fileName:     "product_controller_routes",
			tableName:    "products",
			resourceName: "Product",
		},
		{
			name:         "Should generate routes for admin users controller",
			fileName:     "admin_user_controller_routes",
			tableName:    "admin_users",
			resourceName: "AdminUser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			routesDir := filepath.Join(tempDir, "router", "routes")

			err := os.MkdirAll(routesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create routes directory: %v", err)
			}

			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			templateRenderer := NewTemplateRenderer()
			routeContent, err := templateRenderer.generateRouteContent(
				tt.resourceName,
				tt.tableName,
			)
			if err != nil {
				t.Fatalf("Failed to generate route content: %v", err)
			}

			routesPath := filepath.Join("router", "routes", tt.tableName+".go")

			err = os.WriteFile(routesPath, []byte(routeContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write routes file: %v", err)
			}

			routeGenerator := NewRouteGenerator()
			err = routeGenerator.formatGoFile(routesPath)
			if err != nil {
				t.Fatalf("Failed to format routes file: %v", err)
			}

			formattedRoutesContent, err := os.ReadFile(routesPath)
			if err != nil {
				t.Fatalf("Failed to read formatted routes file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

			g.Assert(t, tt.fileName, formattedRoutesContent)
		})
	}
}

func buildCatalogFromTableMigrations(
	tableName string,
	migrationsDirs []string,
) (*catalog.Catalog, error) {
	allMigrations, err := migrations.DiscoverMigrations(migrationsDirs)
	if err != nil {
		return nil, err
	}

	cat := catalog.NewCatalog("public")
	for _, migration := range allMigrations {
		for _, statement := range migration.Statements {
			if isRelevantForTable(statement, tableName) {
				if err := ddl.ApplyDDL(cat, statement, migration.FilePath, "postgresql"); err != nil {
					return nil, err
				}
			}
		}
	}

	return cat, nil
}

func isRelevantForTable(stmt, targetTable string) bool {
	stmtLower := strings.ToLower(stmt)
	targetLower := strings.ToLower(targetTable)

	if strings.Contains(stmtLower, "create table") &&
		strings.Contains(stmtLower, targetLower) {
		return true
	}

	if strings.Contains(stmtLower, "alter table") &&
		strings.Contains(stmtLower, targetLower) {
		return true
	}

	if strings.Contains(stmtLower, "drop table") &&
		strings.Contains(stmtLower, targetLower) {
		return true
	}

	return false
}

func TestControllerRegistration__GoldenFile(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		resourceName string
	}{
		{
			name:         "Should register User controller",
			fileName:     "user_controller_registration",
			resourceName: "User",
		},
		{
			name:         "Should register Product controller",
			fileName:     "product_controller_registration",
			resourceName: "Product",
		},
		{
			name:         "Should register AdminUser controller",
			fileName:     "admin_user_controller_registration",
			resourceName: "AdminUser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			controllersDir := filepath.Join(tempDir, "controllers")

			err := os.MkdirAll(controllersDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create controllers directory: %v", err)
			}

			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}

			initialControllerGoldenPath := filepath.Join(
				originalWd,
				"testdata",
				"base_controller.go",
			)
			initialControllerContent, err := os.ReadFile(initialControllerGoldenPath)
			if err != nil {
				t.Fatalf("Failed to read initial controller golden file: %v", err)
			}

			controllerFile := filepath.Join(controllersDir, "controller.go")
			err = os.WriteFile(controllerFile, initialControllerContent, 0o644)
			if err != nil {
				t.Fatalf("Failed to create controller.go: %v", err)
			}

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			fileGenerator := NewFileGenerator()
			err = fileGenerator.registerController(tt.resourceName)
			if err != nil {
				t.Fatalf("Failed to register controller: %v", err)
			}

			updatedControllerContent, err := os.ReadFile(controllerFile)
			if err != nil {
				t.Fatalf("Failed to read updated controller file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

			g.Assert(t, tt.fileName, updatedControllerContent)
		})
	}
}

func TestMultipleControllerRegistration__GoldenFile(t *testing.T) {
	t.Run("Should register multiple controllers sequentially", func(t *testing.T) {
		tempDir := t.TempDir()
		controllersDir := filepath.Join(tempDir, "controllers")

		err := os.MkdirAll(controllersDir, constants.DirPermissionDefault)
		if err != nil {
			t.Fatalf("Failed to create controllers directory: %v", err)
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}

		initialControllerGoldenPath := filepath.Join(
			originalWd,
			"testdata",
			"base_controller.go",
		)
		initialControllerContent, err := os.ReadFile(initialControllerGoldenPath)
		if err != nil {
			t.Fatalf("Failed to read initial controller golden file: %v", err)
		}

		controllerFile := filepath.Join(controllersDir, "controller.go")
		err = os.WriteFile(controllerFile, initialControllerContent, 0o644)
		if err != nil {
			t.Fatalf("Failed to create controller.go: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tempDir)

		fileGenerator := NewFileGenerator()

		// Register first controller (User)
		err = fileGenerator.registerController("User")
		if err != nil {
			t.Fatalf("Failed to register User controller: %v", err)
		}

		// Register second controller (Product)
		err = fileGenerator.registerController("Product")
		if err != nil {
			t.Fatalf("Failed to register Product controller: %v", err)
		}

		updatedControllerContent, err := os.ReadFile(controllerFile)
		if err != nil {
			t.Fatalf("Failed to read updated controller file: %v", err)
		}

		fixtureDir := filepath.Join(originalWd, "testdata")
		g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

		g.Assert(t, "multiple_controllers_registration", updatedControllerContent)
	})
}

func TestRouterRegistration__GoldenFile(t *testing.T) {
	t.Run("Should register User routes in separate function", func(t *testing.T) {
		tempDir := t.TempDir()
		routerDir := filepath.Join(tempDir, "router")

		err := os.MkdirAll(routerDir, constants.DirPermissionDefault)
		if err != nil {
			t.Fatalf("Failed to create router directory: %v", err)
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}

		// Set up base registry.go file
		baseRegistryPath := filepath.Join(originalWd, "testdata", "base_router_registry.go")
		baseRegistryContent, err := os.ReadFile(baseRegistryPath)
		if err != nil {
			t.Fatalf("Failed to read base router registry file: %v", err)
		}

		registryFile := filepath.Join(routerDir, "registry.go")
		err = os.WriteFile(registryFile, baseRegistryContent, 0o644)
		if err != nil {
			t.Fatalf("Failed to create registry.go: %v", err)
		}

		// Set up base register.go file
		baseRegisterPath := filepath.Join(originalWd, "testdata", "base_router_register.go")
		baseRegisterContent, err := os.ReadFile(baseRegisterPath)
		if err != nil {
			t.Fatalf("Failed to read base router register file: %v", err)
		}

		registerFile := filepath.Join(routerDir, "register.go")
		err = os.WriteFile(registerFile, baseRegisterContent, 0o644)
		if err != nil {
			t.Fatalf("Failed to create register.go: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tempDir)

		routeGenerator := NewRouteGenerator()
		err = routeGenerator.updateRouterRegister("User", "users")
		if err != nil {
			t.Fatalf("Failed to update router register: %v", err)
		}

		// Check registry.go
		updatedRegistryContent, err := os.ReadFile(registryFile)
		if err != nil {
			t.Fatalf("Failed to read updated router registry file: %v", err)
		}

		// Check register.go
		updatedRegisterContent, err := os.ReadFile(registerFile)
		if err != nil {
			t.Fatalf("Failed to read updated router register file: %v", err)
		}

		fixtureDir := filepath.Join(originalWd, "testdata")
		g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

		g.Assert(t, "user_router_registry_registration", updatedRegistryContent)
		g.Assert(t, "user_router_registration", updatedRegisterContent)
	})
}

func TestMultipleRouterRegistration__GoldenFile(t *testing.T) {
	t.Run("Should register multiple route sets in separate functions", func(t *testing.T) {
		tempDir := t.TempDir()
		routerDir := filepath.Join(tempDir, "router")

		err := os.MkdirAll(routerDir, constants.DirPermissionDefault)
		if err != nil {
			t.Fatalf("Failed to create router directory: %v", err)
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}

		// Set up base registry.go file
		baseRegistryPath := filepath.Join(originalWd, "testdata", "base_router_registry.go")
		baseRegistryContent, err := os.ReadFile(baseRegistryPath)
		if err != nil {
			t.Fatalf("Failed to read base router registry file: %v", err)
		}

		registryFile := filepath.Join(routerDir, "registry.go")
		err = os.WriteFile(registryFile, baseRegistryContent, 0o644)
		if err != nil {
			t.Fatalf("Failed to create registry.go: %v", err)
		}

		// Set up base register.go file
		baseRegisterPath := filepath.Join(originalWd, "testdata", "base_router_register.go")
		baseRegisterContent, err := os.ReadFile(baseRegisterPath)
		if err != nil {
			t.Fatalf("Failed to read base router register file: %v", err)
		}

		registerFile := filepath.Join(routerDir, "register.go")
		err = os.WriteFile(registerFile, baseRegisterContent, 0o644)
		if err != nil {
			t.Fatalf("Failed to create register.go: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tempDir)

		routeGenerator := NewRouteGenerator()

		err = routeGenerator.updateRouterRegister("User", "users")
		if err != nil {
			t.Fatalf("Failed to update router register with User: %v", err)
		}

		err = routeGenerator.updateRouterRegister("Product", "products")
		if err != nil {
			t.Fatalf("Failed to update router register with Product: %v", err)
		}

		// Check registry.go
		updatedRegistryContent, err := os.ReadFile(registryFile)
		if err != nil {
			t.Fatalf("Failed to read updated router registry file: %v", err)
		}

		// Check register.go
		updatedRegisterContent, err := os.ReadFile(registerFile)
		if err != nil {
			t.Fatalf("Failed to read updated router register file: %v", err)
		}

		fixtureDir := filepath.Join(originalWd, "testdata")
		g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

		g.Assert(t, "multiple_router_registry_registrations", updatedRegistryContent)
		g.Assert(t, "multiple_router_registrations", updatedRegisterContent)
	})
}

package models

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/sebdah/goldie/v2"
)

func TestModelFileGeneration__GoldenFile(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		migrationsDir string
		tableName     string
		resourceName  string
		modulePath    string
	}{
		{
			name:          "Should generate model for simple users table",
			fileName:      "simple_user_table_model",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
		},
		{
			name:          "Should generate model for complex table",
			migrationsDir: "complex_table",
			fileName:      "complex_table_model",
			tableName:     "comprehensive_example",
			resourceName:  "ComprehensiveExample",
			modulePath:    "github.com/example/complex",
		},
		{
			name:          "Should generate model for posts with multi-migration",
			fileName:      "posts_multi_migration_model",
			migrationsDir: "posts_multi_migration",
			tableName:     "posts",
			resourceName:  "Post",
			modulePath:    "github.com/example/blog",
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

			generator := NewGenerator("postgresql")

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}

			model, err := generator.Build(cat, Config{
				TableName:    tt.tableName,
				ResourceName: tt.resourceName,
				PackageName:  "models",
				DatabaseType: "postgresql",
				ModulePath:   tt.modulePath,
			})
			if err != nil {
				t.Fatalf("Failed to build model: %v", err)
			}

			templateContent, err := templates.Files.ReadFile("model.tmpl")
			if err != nil {
				t.Fatalf("Failed to read model template: %v", err)
			}

			modelContent, err := generator.GenerateModelFile(model, string(templateContent))
			if err != nil {
				t.Fatalf("Failed to generate model content: %v", err)
			}

			modelPath := filepath.Join("models", strings.ToLower(tt.resourceName)+".go")

			err = os.WriteFile(modelPath, []byte(modelContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write model file: %v", err)
			}

			err = formatGoFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to format model file: %v", err)
			}

			formattedModelContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read formatted model file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

			g.Assert(t, tt.fileName, formattedModelContent)
		})
	}
}

func TestQueriesFileGeneration__GoldenFile(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		migrationsDir string
		tableName     string
		resourceName  string
	}{
		{
			name:          "Should generate SQL for simple users table",
			fileName:      "simple_user_table_queries",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
		},
		{
			name:          "Should generate SQL for complex table",
			migrationsDir: "complex_table",
			fileName:      "complex_table_queries",
			tableName:     "comprehensive_example",
			resourceName:  "ComprehensiveExample",
		},
		{
			name:          "Should generate SQL for posts with multi-migration",
			fileName:      "posts_multi_migration_queries",
			migrationsDir: "posts_multi_migration",
			tableName:     "posts",
			resourceName:  "Post",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			queriesDir := filepath.Join(tempDir, "database", "queries")

			err := os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}
			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			generator := NewGenerator("postgresql")

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}

			table, err := cat.GetTable("", tt.tableName)
			if err != nil {
				t.Fatalf("Failed to get table from catalog: %v", err)
			}

			sqlContent, err := generator.GenerateSQLContent(tt.resourceName, tt.tableName, table)
			if err != nil {
				t.Fatalf("Failed to generate SQL content: %v", err)
			}

			sqlPath := filepath.Join("database", "queries", tt.tableName+".sql")

			err = os.WriteFile(sqlPath, []byte(sqlContent), 0o644)
			if err != nil {
				t.Fatalf("Failed to write SQL file: %v", err)
			}

			queriesContent, err := os.ReadFile(sqlPath)
			if err != nil {
				t.Fatalf("Failed to read formatted model file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".sql"))

			g.Assert(t, tt.fileName, queriesContent)
		})
	}
}

func TestModelRefresh__PreservesCustomCode__GoldenFile(t *testing.T) {
	tests := []struct {
		name                   string
		initialMigrationsDir   string
		refreshMigrationsDir   string
		tableName              string
		resourceName           string
		modulePath             string
		beforeRefreshFixture   string
		afterRefreshFixture    string
	}{
		{
			name:                 "Should preserve custom functions when refreshing User model with schema changes",
			initialMigrationsDir: "simple_user_table",
			refreshMigrationsDir: "simple_user_table_with_phone",
			tableName:            "users",
			resourceName:         "User",
			modulePath:           "github.com/example/myapp",
			beforeRefreshFixture: "user_model_with_custom_code_before_refresh",
			afterRefreshFixture:  "user_model_with_custom_code_after_refresh_with_phone",
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

			initialMigrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.initialMigrationsDir)
			refreshMigrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.refreshMigrationsDir)
			modelPath := filepath.Join(modelsDir, strings.ToLower(tt.resourceName)+".go")
			sqlPath := filepath.Join(queriesDir, tt.tableName+".sql")

			generator := NewGenerator("postgresql")

			// First, generate the initial model
			err = generator.GenerateModelFromMigrations(
				tt.tableName, tt.resourceName,
				[]string{initialMigrationsDir},
				modelPath, sqlPath,
				tt.modulePath,
			)
			if err != nil {
				t.Fatalf("Failed to generate initial model: %v", err)
			}

			// Load the "before refresh" fixture (model with custom code)
			beforeRefreshPath := filepath.Join(originalWd, "testdata", tt.beforeRefreshFixture+".go")
			beforeRefreshContent, err := os.ReadFile(beforeRefreshPath)
			if err != nil {
				t.Fatalf("Failed to read before refresh fixture: %v", err)
			}

			// Write the fixture content to simulate a model with custom code
			err = os.WriteFile(modelPath, beforeRefreshContent, constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to write model file with custom code: %v", err)
			}

			// Now refresh the model with the updated schema
			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{refreshMigrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from refresh migrations: %v", err)
			}

			err = generator.RefreshModel(cat, tt.resourceName, tt.tableName, modelPath, sqlPath, tt.modulePath)
			if err != nil {
				t.Fatalf("Failed to refresh model: %v", err)
			}

			// Read the refreshed content
			refreshedContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read refreshed model file: %v", err)
			}

			// Compare with golden file
			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".go"))

			g.Assert(t, tt.afterRefreshFixture, refreshedContent)
		})
	}
}

func TestSQLRefresh__ReplacesCompleteFile(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		tableName     string
		resourceName  string
	}{
		{
			name:          "Should completely replace SQL file on refresh",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			queriesDir := filepath.Join(tempDir, "database", "queries")

			err := os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)
			sqlPath := filepath.Join(queriesDir, tt.tableName+".sql")

			generator := NewGenerator("postgresql")

			// Create initial SQL file with custom content
			initialSQL := `-- Custom comment that should be replaced
-- name: QueryUserByID :one
select * from users where id=$1;

-- Custom query that should be removed
-- name: CustomQuery :many  
select * from users where custom_field = $1;
`
			err = os.WriteFile(sqlPath, []byte(initialSQL), constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to write initial SQL file: %v", err)
			}

			// Refresh the SQL file
			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}

			err = generator.refreshSQLFile(tt.resourceName, tt.tableName, cat, sqlPath)
			if err != nil {
				t.Fatalf("Failed to refresh SQL file: %v", err)
			}

			// Verify the refreshed SQL content
			refreshedContent, err := os.ReadFile(sqlPath)
			if err != nil {
				t.Fatalf("Failed to read refreshed SQL file: %v", err)
			}

			refreshedStr := string(refreshedContent)

			// Check that custom content is removed (complete replacement)
			if strings.Contains(refreshedStr, "Custom comment that should be replaced") {
				t.Error("Expected custom comment to be removed after SQL refresh")
			}

			if strings.Contains(refreshedStr, "CustomQuery") {
				t.Error("Expected custom query to be removed after SQL refresh")
			}

			// Check that all standard CRUD operations are present
			expectedQueries := []string{
				"-- name: Query" + tt.resourceName + "ByID :one",
				"-- name: Query" + tt.resourceName + "s :many", 
				"-- name: QueryAll" + tt.resourceName + "s :many",
				"-- name: Insert" + tt.resourceName + " :one",
				"-- name: Update" + tt.resourceName + " :one",
				"-- name: Delete" + tt.resourceName + " :exec",
				"-- name: QueryPaginated" + tt.resourceName + "s :many",
				"-- name: Count" + tt.resourceName + "s :one",
			}

			for _, expectedQuery := range expectedQueries {
				if !strings.Contains(refreshedStr, expectedQuery) {
					t.Errorf("Expected query %s to be present after SQL refresh, but it was not found", expectedQuery)
				}
			}
		})
	}
}

func formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

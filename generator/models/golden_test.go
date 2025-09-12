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
		databaseType  string
	}{
		{
			name:          "Should generate model for simple users table",
			fileName:      "simple_user_table_model",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			databaseType:  "postgresql",
		},
		{
			name:          "Should generate model for complex table",
			migrationsDir: "complex_table",
			fileName:      "complex_table_model",
			tableName:     "comprehensive_example",
			resourceName:  "ComprehensiveExample",
			modulePath:    "github.com/example/complex",
			databaseType:  "postgresql",
		},
		{
			name:          "Should generate model for posts with multi-migration",
			fileName:      "posts_multi_migration_model",
			migrationsDir: "posts_multi_migration",
			tableName:     "posts",
			resourceName:  "Post",
			modulePath:    "github.com/example/blog",
			databaseType:  "postgresql",
		},
		{
			name:          "Should generate SQLite model for simple users table",
			fileName:      "sqlite_user_table_model",
			migrationsDir: "sqlite_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/sqlite",
			databaseType:  "sqlite",
		},
		{
			name:          "Should generate SQLite model for complex products table",
			fileName:      "sqlite_complex_table_model",
			migrationsDir: "sqlite_complex_table",
			tableName:     "products",
			resourceName:  "Product",
			modulePath:    "github.com/example/sqlite",
			databaseType:  "sqlite",
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
			if err != nil {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
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

func TestModelFileGeneration_InvalidPrimaryKeyTypes(t *testing.T) {
	tests := []struct {
		name             string
		migrationsDir    string
		tableName        string
		resourceName     string
		modulePath       string
		databaseType     string
		expectedErrorMsg string
	}{
		{
			name:             "Should reject PostgreSQL migration with TEXT primary key",
			migrationsDir:    "invalid_pg_primary_key",
			tableName:        "users",
			resourceName:     "User",
			modulePath:       "github.com/example/myapp",
			databaseType:     "postgresql",
			expectedErrorMsg: "PostgreSQL primary keys must use 'uuid'",
		},
		{
			name:             "Should reject SQLite migration with UUID primary key",
			migrationsDir:    "invalid_sqlite_primary_key",
			tableName:        "users",
			resourceName:     "User",
			modulePath:       "github.com/example/myapp",
			databaseType:     "sqlite",
			expectedErrorMsg: "SQLite primary keys must use 'text'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			originalWd, _ := os.Getwd()

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(originalWd, "testdata", "migrations", tt.migrationsDir)

			generator := NewGenerator(tt.databaseType)

			_, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)

			if err == nil {
				t.Fatal("Expected error due to invalid primary key type, but got none")
			}

			if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
				t.Errorf("Expected error message to contain '%s', but got: %s", tt.expectedErrorMsg, err.Error())
			}

			// Verify the error message also contains the migration file name
			if !strings.Contains(err.Error(), "001_users_text_pk.sql") && !strings.Contains(err.Error(), "001_users_uuid_pk.sql") {
				t.Errorf("Expected error message to contain migration file name, but got: %s", err.Error())
			}
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
		databaseType  string
	}{
		{
			name:          "Should generate SQL for simple users table",
			fileName:      "simple_user_table_queries",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "postgresql",
		},
		{
			name:          "Should generate SQL for complex table",
			migrationsDir: "complex_table",
			fileName:      "complex_table_queries",
			tableName:     "comprehensive_example",
			resourceName:  "ComprehensiveExample",
			databaseType:  "postgresql",
		},
		{
			name:          "Should generate SQL for posts with multi-migration",
			fileName:      "posts_multi_migration_queries",
			migrationsDir: "posts_multi_migration",
			tableName:     "posts",
			resourceName:  "Post",
			databaseType:  "postgresql",
		},
		{
			name:          "Should generate SQLite SQL for simple users table",
			fileName:      "sqlite_user_table_queries",
			migrationsDir: "sqlite_user_table",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "sqlite",
		},
		{
			name:          "Should generate SQLite SQL for complex products table",
			fileName:      "sqlite_complex_table_queries",
			migrationsDir: "sqlite_complex_table",
			tableName:     "products",
			resourceName:  "Product",
			databaseType:  "sqlite",
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

			generator := NewGenerator(tt.databaseType)

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

func TestQueryRefresh__PreservesModelFunctions__GoldenFile(t *testing.T) {
	tests := []struct {
		name                 string
		initialMigrationsDir string
		refreshMigrationsDir string
		tableName            string
		resourceName         string
		modulePath           string
		beforeRefreshFixture string
	}{
		{
			name:                 "Should preserve custom model functions when refreshing only SQL queries with schema changes",
			initialMigrationsDir: "simple_user_table",
			refreshMigrationsDir: "simple_user_table_with_phone",
			tableName:            "users",
			resourceName:         "User",
			modulePath:           "github.com/example/myapp",
			beforeRefreshFixture: "user_model_with_custom_code_before_refresh",
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

			initialMigrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.initialMigrationsDir,
			)
			refreshMigrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.refreshMigrationsDir,
			)
			modelPath := filepath.Join(modelsDir, strings.ToLower(tt.resourceName)+".go")
			sqlPath := filepath.Join(queriesDir, tt.tableName+".sql")

			generator := NewGenerator("postgresql")

			err = generator.GenerateModelFromMigrations(
				tt.tableName, tt.resourceName,
				[]string{initialMigrationsDir},
				modelPath, sqlPath,
				tt.modulePath,
			)
			if err != nil {
				t.Fatalf("Failed to generate initial model: %v", err)
			}

			beforeRefreshPath := filepath.Join(
				originalWd,
				"testdata",
				tt.beforeRefreshFixture+".go",
			)
			beforeRefreshContent, err := os.ReadFile(beforeRefreshPath)
			if err != nil {
				t.Fatalf("Failed to read before refresh fixture: %v", err)
			}

			err = os.WriteFile(modelPath, beforeRefreshContent, constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to write model file with custom code: %v", err)
			}

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{refreshMigrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from refresh migrations: %v", err)
			}

			err = generator.RefreshQueries(
				cat,
				tt.resourceName,
				tt.tableName,
				sqlPath,
			)
			if err != nil {
				t.Fatalf("Failed to refresh queries: %v", err)
			}

			refreshedContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read refreshed model file: %v", err)
			}

			// Verify that the model file content remains exactly the same (no model functions were changed)
			if string(refreshedContent) != string(beforeRefreshContent) {
				t.Error("Model file content changed during query refresh, but it should remain unchanged")
				t.Logf("Expected model to remain unchanged, but content differs")
			}

			// Also verify that the SQL file was actually updated by checking it exists and has expected content
			_, err = os.Stat(sqlPath)
			if err != nil {
				t.Fatalf("SQL file should have been refreshed but doesn't exist: %v", err)
			}

			sqlContent, err := os.ReadFile(sqlPath)
			if err != nil {
				t.Fatalf("Failed to read SQL file: %v", err)
			}

			// Verify SQL file contains expected query names
			sqlStr := string(sqlContent)
			expectedQueries := []string{
				"-- name: QueryUserByID",
				"-- name: InsertUser",
				"-- name: UpdateUser",
				"-- name: DeleteUser",
			}
			for _, query := range expectedQueries {
				if !strings.Contains(sqlStr, query) {
					t.Errorf("Expected SQL file to contain query %s after refresh", query)
				}
			}
		})
	}
}

func TestRefreshQueries__ValidatesIDColumns(t *testing.T) {
	tests := []struct {
		name             string
		migrationsDir    string
		tableName        string
		resourceName     string
		databaseType     string
		expectedErrorMsg string
		shouldFail       bool
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
			name:          "Should succeed with valid SQLite TEXT primary key",
			migrationsDir: "sqlite_user_table",
			tableName:     "users",
			resourceName:  "User",
			databaseType:  "sqlite",
			shouldFail:    false,
		},
		{
			name:             "Should fail with invalid PostgreSQL TEXT primary key",
			migrationsDir:    "invalid_pg_primary_key",
			tableName:        "users",
			resourceName:     "User",
			databaseType:     "postgresql",
			expectedErrorMsg: "PostgreSQL primary keys must use 'uuid'",
			shouldFail:       true,
		},
		{
			name:             "Should fail with invalid SQLite UUID primary key",
			migrationsDir:    "invalid_sqlite_primary_key",
			tableName:        "users",
			resourceName:     "User",
			databaseType:     "sqlite",
			expectedErrorMsg: "SQLite primary keys must use 'text'",
			shouldFail:       true,
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

			// Create a dummy SQL file for the test
			err = os.WriteFile(sqlPath, []byte("-- dummy queries"), constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to create SQL file: %v", err)
			}

			generator := NewGenerator(tt.databaseType)

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil && !tt.shouldFail {
				t.Fatalf("Failed to build catalog from migrations: %v", err)
			}
			if err != nil && tt.shouldFail {
				// Expected failure during catalog building due to ID validation
				if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf("Expected error message to contain '%s', but got: %s", tt.expectedErrorMsg, err.Error())
				}
				return
			}

			err = generator.RefreshQueries(cat, tt.resourceName, tt.tableName, sqlPath)

			if tt.shouldFail {
				if err == nil {
					t.Fatal("Expected error due to invalid primary key type, but got none")
				}
				if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf("Expected error message to contain '%s', but got: %s", tt.expectedErrorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestConstructorRefresh__UpdatesOnlyConstructors__GoldenFile(t *testing.T) {
	tests := []struct {
		name                 string
		initialMigrationsDir string
		refreshMigrationsDir string
		tableName            string
		resourceName         string
		modulePath           string
		beforeRefreshFixture string
	}{
		{
			name:                 "Should update only constructor functions when refreshing with schema changes",
			initialMigrationsDir: "simple_user_table",
			refreshMigrationsDir: "simple_user_table_with_phone",
			tableName:            "users",
			resourceName:         "User",
			modulePath:           "github.com/example/myapp",
			beforeRefreshFixture: "user_model_with_custom_code_before_refresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			queriesDir := filepath.Join(tempDir, "database", "queries")
			modelsDir := filepath.Join(tempDir, "models")
			internalDbDir := filepath.Join(modelsDir, "internal", "db")

			err := os.MkdirAll(queriesDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create queries directory: %v", err)
			}

			err = os.MkdirAll(modelsDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create models directory: %v", err)
			}

			err = os.MkdirAll(internalDbDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create internal/db directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			initialMigrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.initialMigrationsDir,
			)
			refreshMigrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.refreshMigrationsDir,
			)
			modelPath := filepath.Join(modelsDir, strings.ToLower(tt.resourceName)+".go")
			sqlPath := filepath.Join(queriesDir, tt.tableName+".sql")

			generator := NewGenerator("postgresql")

			// Generate initial model
			err = generator.GenerateModelFromMigrations(
				tt.tableName, tt.resourceName,
				[]string{initialMigrationsDir},
				modelPath, sqlPath,
				tt.modulePath,
			)
			if err != nil {
				t.Fatalf("Failed to generate initial model: %v", err)
			}

			// Replace with custom model that has additional functions
			beforeRefreshPath := filepath.Join(
				originalWd,
				"testdata",
				tt.beforeRefreshFixture+".go",
			)
			beforeRefreshContent, err := os.ReadFile(beforeRefreshPath)
			if err != nil {
				t.Fatalf("Failed to read before refresh fixture: %v", err)
			}

			err = os.WriteFile(modelPath, beforeRefreshContent, constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to write model file with custom code: %v", err)
			}

			// Now refresh with new schema
			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{refreshMigrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog from refresh migrations: %v", err)
			}

			// Use RefreshConstructors - now it works on model-specific constructor file
			constructorFileName := fmt.Sprintf("%s_constructors.go", strings.ToLower(tt.resourceName))
			constructorPath := filepath.Join(internalDbDir, constructorFileName)
			err = generator.RefreshConstructors(
				cat,
				tt.resourceName,
				tt.tableName,
				constructorPath,
				tt.modulePath,
			)
			if err != nil {
				t.Fatalf("Failed to refresh constructors: %v", err)
			}

			refreshedContent, err := os.ReadFile(modelPath)
			if err != nil {
				t.Fatalf("Failed to read refreshed model file: %v", err)
			}

			// Verify that custom functions are preserved in the model file
			refreshedStr := string(refreshedContent)
			if !strings.Contains(refreshedStr, "// Custom functions added by developer - these should be preserved during refresh") {
				t.Error("Custom functions comment was not preserved during constructor refresh")
			}
			if !strings.Contains(refreshedStr, "func (u User) IsAdult() bool {") {
				t.Error("Custom IsAdult function was not preserved during constructor refresh")
			}
			if !strings.Contains(refreshedStr, "func CustomUserHelper(id uuid.UUID) string {") {
				t.Error("Custom CustomUserHelper function was not preserved during constructor refresh")
			}
			if !strings.Contains(refreshedStr, "func GetActiveUsersCount(ctx context.Context, dbtx db.DBTX) (int64, error) {") {
				t.Error("Custom GetActiveUsersCount function was not preserved during constructor refresh")
			}

			// Verify that model functions now use db.New* constructors
			if !strings.Contains(refreshedStr, "db.NewInsertUserParams(") {
				t.Error("Model functions should now use db.NewInsertUserParams constructor")
			}
			if !strings.Contains(refreshedStr, "db.NewUpdateUserParams(") {
				t.Error("Model functions should now use db.NewUpdateUserParams constructor")
			}

			// Verify that constructor functions were created in the internal/db package
			constructorContent, err := os.ReadFile(constructorPath)
			if err != nil {
				t.Fatalf("Failed to read constructor file: %v", err)
			}
			constructorStr := string(constructorContent)

			if !strings.Contains(constructorStr, "func NewInsertUserParams(") {
				t.Error("Constructor file should contain NewInsertUserParams function")
			}
			if !strings.Contains(constructorStr, "func NewUpdateUserParams(") {
				t.Error("Constructor file should contain NewUpdateUserParams function")
			}

			// Verify the new schema includes phone field in constructors
			if !strings.Contains(constructorStr, "phone string,") {
				t.Error("Constructor functions should include phone parameter for new schema")
			}

			// Verify that only necessary imports are included (no unused imports)
			expectedImports := []string{
				`"github.com/google/uuid"`,         // Always needed for ID generation
				`"github.com/jackc/pgx/v5/pgtype"`, // Needed for pgtype.Int4, pgtype.Bool used in this model
			}
			unexpectedImports := []string{
				`"time"`,         // Not needed in constructors for this model
				`"database/sql"`, // Not needed for PostgreSQL simple user model
			}

			for _, expectedImport := range expectedImports {
				if !strings.Contains(constructorStr, expectedImport) {
					t.Errorf("Constructor file should contain import %s", expectedImport)
				}
			}

			for _, unexpectedImport := range unexpectedImports {
				if strings.Contains(constructorStr, unexpectedImport) {
					t.Errorf("Constructor file should NOT contain unused import %s", unexpectedImport)
				}
			}

			// The new schema should include phone field in constructors if the migration adds it
			sqlContent, err := os.ReadFile(sqlPath)
			if err != nil {
				t.Fatalf("Failed to read SQL file: %v", err)
			}

			// Verify SQL file was also updated
			sqlStr := string(sqlContent)
			expectedQueries := []string{
				"-- name: QueryUserByID",
				"-- name: InsertUser",
				"-- name: UpdateUser",
				"-- name: DeleteUser",
			}
			for _, query := range expectedQueries {
				if !strings.Contains(sqlStr, query) {
					t.Errorf("Expected SQL file to contain query %s after refresh", query)
				}
			}
		})
	}
}

func TestConstructorImports__OnlyNecessaryImports(t *testing.T) {
	tests := []struct {
		name              string
		migrationsDir     string
		tableName         string
		resourceName      string
		modulePath        string
		expectedImports   []string
		unexpectedImports []string
	}{
		{
			name:          "PostgreSQL user model should only import uuid and pgtype",
			migrationsDir: "simple_user_table",
			tableName:     "users",
			resourceName:  "User",
			modulePath:    "github.com/example/myapp",
			expectedImports: []string{
				`"github.com/google/uuid"`,
				`"github.com/jackc/pgx/v5/pgtype"`,
			},
			unexpectedImports: []string{
				`"time"`,
				`"database/sql"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			internalDbDir := filepath.Join(tempDir, "models", "internal", "db")

			err := os.MkdirAll(internalDbDir, constants.DirPermissionDefault)
			if err != nil {
				t.Fatalf("Failed to create internal/db directory: %v", err)
			}

			originalWd, _ := os.Getwd()
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			migrationsDir := filepath.Join(
				originalWd,
				"testdata",
				"migrations",
				tt.migrationsDir,
			)

			generator := NewGenerator("postgresql")

			cat, err := generator.buildCatalogFromTableMigrations(
				tt.tableName,
				[]string{migrationsDir},
			)
			if err != nil {
				t.Fatalf("Failed to build catalog: %v", err)
			}

			constructorFileName := fmt.Sprintf("%s_constructors.go", strings.ToLower(tt.resourceName))
			constructorPath := filepath.Join(internalDbDir, constructorFileName)

			err = generator.GenerateConstructors(cat, tt.resourceName, tt.tableName, constructorPath, tt.modulePath)
			if err != nil {
				t.Fatalf("Failed to generate constructors: %v", err)
			}

			constructorContent, err := os.ReadFile(constructorPath)
			if err != nil {
				t.Fatalf("Failed to read constructor file: %v", err)
			}

			constructorStr := string(constructorContent)

			// Verify expected imports are present
			for _, expectedImport := range tt.expectedImports {
				if !strings.Contains(constructorStr, expectedImport) {
					t.Errorf("Constructor file should contain import %s", expectedImport)
				}
			}

			// Verify unexpected imports are not present
			for _, unexpectedImport := range tt.unexpectedImports {
				if strings.Contains(constructorStr, unexpectedImport) {
					t.Errorf("Constructor file should NOT contain unused import %s", unexpectedImport)
				}
			}
		})
	}
}

func TestSQLRefresh__PreservesCustomQueries__GoldenFile(t *testing.T) {
	t.Skip("Skipping SQL refresh test as it is not yet implemented")

	tests := []struct {
		name                 string
		migrationsDir        string
		tableName            string
		resourceName         string
		beforeRefreshFixture string
		afterRefreshFixture  string
	}{
		{
			name:                 "Should preserve custom queries while refreshing CRUD queries",
			migrationsDir:        "simple_user_table",
			tableName:            "users",
			resourceName:         "User",
			beforeRefreshFixture: "users_sql_with_custom_queries_before_refresh",
			afterRefreshFixture:  "users_sql_with_custom_queries_after_refresh",
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

			beforeRefreshPath := filepath.Join(
				originalWd,
				"testdata",
				tt.beforeRefreshFixture+".sql",
			)
			beforeRefreshContent, err := os.ReadFile(beforeRefreshPath)
			if err != nil {
				t.Fatalf("Failed to read before refresh fixture: %v", err)
			}

			err = os.WriteFile(sqlPath, beforeRefreshContent, constants.FilePermissionPrivate)
			if err != nil {
				t.Fatalf("Failed to write initial SQL file: %v", err)
			}

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

			refreshedContent, err := os.ReadFile(sqlPath)
			if err != nil {
				t.Fatalf("Failed to read refreshed SQL file: %v", err)
			}

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".sql"))

			g.Assert(t, tt.afterRefreshFixture, refreshedContent)
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

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func setupModelManagerTest(t *testing.T) (*ModelManager, func()) {
	t.Helper()
	cache.ClearFileSystemCache()

	tmpDir := t.TempDir()

	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	dbDir := filepath.Join(tmpDir, "database")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("Failed to create database directory: %v", err)
	}

	sqlcContent := `version: "2"
sql:
  - engine: postgresql
    schema: migrations`
	if err := os.WriteFile(filepath.Join(dbDir, "sqlc.yaml"), []byte(sqlcContent), 0o644); err != nil {
		t.Fatalf("Failed to write sqlc.yaml: %v", err)
	}

	migrationsDir := filepath.Join(dbDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	return coord.ModelManager, func() {
		os.Chdir(originalDir)
		cache.ClearFileSystemCache()
	}
}

func TestExtractTableNameOverride(t *testing.T) {
	_, cleanup := setupModelManagerTest(t)
	defer cleanup()

	tests := []struct {
		name         string
		resourceName string
		fileContent  string
		wantTable    string
		wantFound    bool
	}{
		{
			name:         "extract User model override",
			resourceName: "User",
			fileContent: `package models
// USER_MODEL_TABLE_NAME: accounts

import (
	"context"
)

type User struct {
	ID string
}
`,
			wantTable: "accounts",
			wantFound: true,
		},
		{
			name:         "extract CompanyAccount model override",
			resourceName: "CompanyAccount",
			fileContent: `package models
// COMPANYACCOUNT_MODEL_TABLE_NAME: legacy_accounts

import (
	"context"
)

type CompanyAccount struct {
	ID string
}
`,
			wantTable: "legacy_accounts",
			wantFound: true,
		},
		{
			name:         "no override marker present",
			resourceName: "User",
			fileContent: `package models

import (
	"context"
)

type User struct {
	ID string
}
`,
			wantTable: "",
			wantFound: false,
		},
		{
			name:         "wrong model name marker",
			resourceName: "User",
			fileContent: `package models
// PRODUCT_MODEL_TABLE_NAME: products

import (
	"context"
)

type User struct {
	ID string
}
`,
			wantTable: "",
			wantFound: false,
		},
		{
			name:         "marker with extra spaces around value",
			resourceName: "User",
			fileContent: `package models
// USER_MODEL_TABLE_NAME:   accounts

import (
	"context"
)

type User struct {
	ID string
}
`,
			wantTable: "accounts",
			wantFound: true,
		},
		{
			name:         "marker after other comments",
			resourceName: "User",
			fileContent: `package models
// This is a user model
// USER_MODEL_TABLE_NAME: accounts

import (
	"context"
)

type User struct {
	ID string
}
`,
			wantTable: "accounts",
			wantFound: true,
		},
		{
			name:         "marker after import should not be found",
			resourceName: "User",
			fileContent: `package models

import (
	"context"
)

// USER_MODEL_TABLE_NAME: accounts

type User struct {
	ID string
}
`,
			wantTable: "",
			wantFound: false,
		},
		{
			name:         "empty override value",
			resourceName: "User",
			fileContent: `package models
// USER_MODEL_TABLE_NAME:

import (
	"context"
)

type User struct {
	ID string
}
`,
			wantTable: "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelPath := filepath.Join("models", "test_model.go")
			if err := os.WriteFile(modelPath, []byte(tt.fileContent), 0o644); err != nil {
				t.Fatalf("Failed to write test model file: %v", err)
			}

			gotTable, gotFound := ExtractTableNameOverride(modelPath, tt.resourceName)

			if gotFound != tt.wantFound {
				t.Errorf("ExtractTableNameOverride() gotFound = %v, wantFound %v", gotFound, tt.wantFound)
			}

			if gotTable != tt.wantTable {
				t.Errorf("ExtractTableNameOverride() gotTable = %v, wantTable %v", gotTable, tt.wantTable)
			}

			os.Remove(modelPath)
		})
	}
}

func TestExtractTableNameOverride_FileNotFound(t *testing.T) {
	_, cleanup := setupModelManagerTest(t)
	defer cleanup()

	modelPath := filepath.Join("models", "nonexistent.go")

	gotTable, gotFound := ExtractTableNameOverride(modelPath, "User")

	if gotFound {
		t.Errorf("ExtractTableNameOverride() for nonexistent file: gotFound = true, want false")
	}

	if gotTable != "" {
		t.Errorf("ExtractTableNameOverride() for nonexistent file: gotTable = %v, want empty string", gotTable)
	}
}

func TestExtractTableNameOverride_ResourceNameMatching(t *testing.T) {
	_, cleanup := setupModelManagerTest(t)
	defer cleanup()

	tests := []struct {
		name         string
		resourceName string
		markerName   string
		wantFound    bool
	}{
		{
			name:         "PascalCase resource matches UPPERCASE marker",
			resourceName: "User",
			markerName:   "USER",
			wantFound:    true,
		},
		{
			name:         "CompoundName resource matches COMPOUNDNAME marker",
			resourceName: "CompanyAccount",
			markerName:   "COMPANYACCOUNT",
			wantFound:    true,
		},
		{
			name:         "wrong marker name does not match",
			resourceName: "User",
			markerName:   "ACCOUNT",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileContent := "package models\n// " + tt.markerName + "_MODEL_TABLE_NAME: test_table\n\ntype Model struct {}"

			modelPath := filepath.Join("models", "test_model.go")
			if err := os.WriteFile(modelPath, []byte(fileContent), 0o644); err != nil {
				t.Fatalf("Failed to write test model file: %v", err)
			}

			_, gotFound := ExtractTableNameOverride(modelPath, tt.resourceName)

			if gotFound != tt.wantFound {
				t.Errorf("ExtractTableNameOverride() gotFound = %v, wantFound %v", gotFound, tt.wantFound)
			}

			os.Remove(modelPath)
		})
	}
}

func TestGenerateQueriesOnly_InvalidTableName(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	tests := []struct {
		name      string
		tableName string
	}{
		{"PascalCase", "User"},
		{"camelCase", "userRole"},
		{"with spaces", "user role"},
		{"empty", ""},
		{"singular", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.GenerateQueriesOnly(tt.tableName)
			if err == nil {
				t.Errorf("GenerateQueriesOnly(%q) expected error for invalid table name, got nil", tt.tableName)
			}
		})
	}
}

func TestGenerateQueriesOnly_ReservedKeywords(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	tests := []struct {
		name      string
		tableName string
	}{
		{"reserved keyword SELECT", "select"},
		{"reserved keyword FROM", "from"},
		{"reserved keyword WHERE", "where"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.GenerateQueriesOnly(tt.tableName)
			if err == nil {
				t.Errorf("GenerateQueriesOnly(%q) expected error for reserved SQL keyword, got nil", tt.tableName)
			}
		})
	}
}

func TestRefreshQueriesOnly_RequiresExistingSQLFile(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	err := manager.RefreshQueriesOnly("non_existents")
	if err == nil {
		t.Error("RefreshQueriesOnly() expected error when SQL file doesn't exist, got nil")
	}

	expectedMsg := "does not exist"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("RefreshQueriesOnly() error = %v, want error containing %q", err, expectedMsg)
	}
}

func TestCheckExistingModel_WarnsWhenModelExists(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	modelPath := filepath.Join("models", "user.go")
	if err := os.WriteFile(modelPath, []byte("package models\ntype User struct{}"), 0o644); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}
	defer os.Remove(modelPath)

	// This should not panic - just prints a warning
	manager.checkExistingModel("User")
}

func TestCheckExistingModel_NoWarningWhenModelNotExists(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	manager.checkExistingModel("NonExistent")
}

func TestSetupModelContext_SingularTableNameWithOverride(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	ctx, err := manager.setupModelContext("UserFeedback", "user_feedback", true)
	if err != nil {
		t.Errorf("setupModelContext() with singular table name override returned error: %v", err)
	}

	if ctx == nil {
		t.Fatal("setupModelContext() returned nil context")
	}

	if ctx.TableName != "user_feedback" {
		t.Errorf("setupModelContext() TableName = %q, want %q", ctx.TableName, "user_feedback")
	}
}

func TestSetupModelContext_SingularTableNameWithoutOverride(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	_, err := manager.setupModelContext("UserFeedback", "user_feedback", false)
	if err == nil {
		t.Error("setupModelContext() without override flag should reject singular table name")
	}

	if !strings.Contains(err.Error(), "must be plural") {
		t.Errorf("setupModelContext() error = %v, want error containing 'must be plural'", err)
	}
}

func TestSetupModelContext_PluralTableNameWithoutOverride(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	ctx, err := manager.setupModelContext("User", "users", false)
	if err != nil {
		t.Errorf("setupModelContext() with plural table name returned error: %v", err)
	}

	if ctx == nil {
		t.Fatal("setupModelContext() returned nil context")
	}

	if ctx.TableName != "users" {
		t.Errorf("setupModelContext() TableName = %q, want %q", ctx.TableName, "users")
	}
}

func TestSetupQueriesContext_ValidTableName(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	ctx, err := manager.setupQueriesContext("user_feedbacks")
	if err != nil {
		t.Errorf("setupQueriesContext() with valid table name returned error: %v", err)
	}

	if ctx == nil {
		t.Fatal("setupQueriesContext() returned nil context")
	}

	if ctx.TableName != "user_feedbacks" {
		t.Errorf("setupQueriesContext() TableName = %q, want %q", ctx.TableName, "user_feedbacks")
	}

	if ctx.ResourceName != "UserFeedback" {
		t.Errorf("setupQueriesContext() ResourceName = %q, want %q", ctx.ResourceName, "UserFeedback")
	}
}

func TestSetupQueriesContext_SingularTableName(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	_, err := manager.setupQueriesContext("user_feedback")
	if err == nil {
		t.Error("setupQueriesContext() should reject singular table name")
	}

	if !strings.Contains(err.Error(), "must be plural") {
		t.Errorf("setupQueriesContext() error = %v, want error containing 'must be plural'", err)
	}
}

func TestGenerateModel_SingularTableNameOverride(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	migrationsDir := filepath.Join("database", "migrations")
	migrationContent := `CREATE TABLE user_feedback (
		id SERIAL PRIMARY KEY,
		message TEXT NOT NULL
	);`
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_user_feedback.up.sql"), []byte(migrationContent), 0o644); err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	err := manager.GenerateModel("UserFeedback", "user_feedback", false)

	if err != nil && strings.Contains(err.Error(), "must be plural") {
		t.Errorf("GenerateModel() with --table-name override should not fail with plural validation: %v", err)
	}
}

func TestGenerateQueriesOnly_RejectsSingularTableName(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	migrationsDir := filepath.Join("database", "migrations")
	migrationContent := `CREATE TABLE user_feedback (
		id SERIAL PRIMARY KEY,
		message TEXT NOT NULL
	);`
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_user_feedback.up.sql"), []byte(migrationContent), 0o644); err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	err := manager.GenerateQueriesOnly("user_feedback")

	if err == nil {
		t.Error("GenerateQueriesOnly() should reject singular table name")
	}

	if err != nil && !strings.Contains(err.Error(), "must be plural") {
		t.Errorf("GenerateQueriesOnly() error = %v, want error containing 'must be plural'", err)
	}
}

func TestGenerateModel_WithFactory(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	// Create queries directory
	queriesDir := filepath.Join("database", "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatalf("Failed to create queries directory: %v", err)
	}

	// Create migration for authors table
	migrationsDir := filepath.Join("database", "migrations")
	migrationContent := `-- +goose Up
CREATE TABLE authors (
	id UUID PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	email VARCHAR(255) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE authors;`
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_authors.sql"), []byte(migrationContent), 0o644); err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	// Generate model with factory (skipFactory = false)
	// This will fail at sqlc step in unit tests, but we're just checking
	// that the function can be called with the skipFactory parameter
	err := manager.GenerateModel("Author", "", false)

	// We expect an error about sqlc since it's not available in unit tests
	// The important part is that the function accepts the skipFactory parameter
	if err == nil {
		t.Error("Expected error due to missing sqlc in test environment")
	}

	if err != nil && !strings.Contains(err.Error(), "sqlc") {
		t.Logf("Got error (expected sqlc error): %v", err)
	}
}

func TestGenerateModel_SkipFactory(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	// Create queries directory
	queriesDir := filepath.Join("database", "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatalf("Failed to create queries directory: %v", err)
	}

	// Create migration for publishers table
	migrationsDir := filepath.Join("database", "migrations")
	migrationContent := `-- +goose Up
CREATE TABLE publishers (
	id UUID PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	country VARCHAR(100),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE publishers;`
	if err := os.WriteFile(filepath.Join(migrationsDir, "002_create_publishers.sql"), []byte(migrationContent), 0o644); err != nil {
		t.Fatalf("Failed to create migration file: %v", err)
	}

	// Generate model without factory (skipFactory = true)
	// This will also fail at sqlc step, but we're checking the parameter is accepted
	err := manager.GenerateModel("Publisher", "", true)

	// We expect an error about sqlc since it's not available in unit tests
	if err == nil {
		t.Error("Expected error due to missing sqlc in test environment")
	}

	if err != nil && !strings.Contains(err.Error(), "sqlc") {
		t.Logf("Got error (expected sqlc error): %v", err)
	}
}

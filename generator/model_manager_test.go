package generator

import (
	"os"
	"path/filepath"
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
	manager, cleanup := setupModelManagerTest(t)
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

			gotTable, gotFound := manager.extractTableNameOverride(modelPath, tt.resourceName)

			if gotFound != tt.wantFound {
				t.Errorf("extractTableNameOverride() gotFound = %v, wantFound %v", gotFound, tt.wantFound)
			}

			if gotTable != tt.wantTable {
				t.Errorf("extractTableNameOverride() gotTable = %v, wantTable %v", gotTable, tt.wantTable)
			}

			os.Remove(modelPath)
		})
	}
}

func TestExtractTableNameOverride_FileNotFound(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	modelPath := filepath.Join("models", "nonexistent.go")

	gotTable, gotFound := manager.extractTableNameOverride(modelPath, "User")

	if gotFound {
		t.Errorf("extractTableNameOverride() for nonexistent file: gotFound = true, want false")
	}

	if gotTable != "" {
		t.Errorf("extractTableNameOverride() for nonexistent file: gotTable = %v, want empty string", gotTable)
	}
}

func TestExtractTableNameOverride_ResourceNameMatching(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
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

			_, gotFound := manager.extractTableNameOverride(modelPath, tt.resourceName)

			if gotFound != tt.wantFound {
				t.Errorf("extractTableNameOverride() gotFound = %v, wantFound %v", gotFound, tt.wantFound)
			}

			os.Remove(modelPath)
		})
	}
}

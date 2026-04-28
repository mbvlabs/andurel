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

	migrationsDir := filepath.Join(tmpDir, "database", "migrations")
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
			name:         "no override comment",
			resourceName: "Server",
			fileContent: `package models

import (
	"context"
)

type Server struct {
	ID string
}
`,
			wantTable: "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "model.go")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0o644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			gotTable, gotFound := extractTableNameOverride(tmpFile, tt.resourceName)
			if gotFound != tt.wantFound {
				t.Errorf("extractTableNameOverride() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotTable != tt.wantTable {
				t.Errorf("extractTableNameOverride() table = %v, want %v", gotTable, tt.wantTable)
			}
		})
	}
}

func TestSetupModelContext(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	t.Run("validates empty resource name", func(t *testing.T) {
		_, err := manager.setupModelContext("", "users", false)
		if err == nil {
			t.Error("Expected error for empty resource name")
		}
	})

	t.Run("creates valid context", func(t *testing.T) {
		ctx, err := manager.setupModelContext("User", "users", false)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ctx.ResourceName != "User" {
			t.Errorf("Expected User, got %s", ctx.ResourceName)
		}
		if ctx.TableName != "users" {
			t.Errorf("Expected users, got %s", ctx.TableName)
		}
		if ctx.PluralName != "users" {
			t.Errorf("Expected users, got %s", ctx.PluralName)
		}
	})

	t.Run("handles table name override", func(t *testing.T) {
		ctx, err := manager.setupModelContext("User", "accounts", true)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ctx.TableName != "accounts" {
			t.Errorf("Expected accounts, got %s", ctx.TableName)
		}
	})
}

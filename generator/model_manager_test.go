package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestEnsureLineInBlock(t *testing.T) {
	src := "package models\n\ntype (\n\tuser struct{}\n\ttoken struct{}\n)\n\nvar (\n\tUser user\n\tToken token\n)\n"

	got := ensureLineInBlock(src, "type (", "\tserver struct{}")
	if !strings.Contains(got, "\tserver struct{}\n)") {
		t.Errorf("expected server struct{} inserted before ); got:\n%s", got)
	}

	// idempotent
	again := ensureLineInBlock(got, "type (", "\tserver struct{}")
	if again != got {
		t.Errorf("expected idempotent insert, but content changed")
	}

	got2 := ensureLineInBlock(got, "var (", "\tServer server")
	if !strings.Contains(got2, "\tServer server\n)") {
		t.Errorf("expected Server server inserted into var block; got:\n%s", got2)
	}
}

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

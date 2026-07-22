package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/models"
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
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
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

func TestPlanModelReturnsCompleteFormattedOutputWithoutWriting(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	writeModelPlanningFixture(t, root)
	originalRegistry, err := os.ReadFile(filepath.Join(root, "models", "model.go"))
	if err != nil {
		t.Fatalf("read original registry: %v", err)
	}
	originalWorkingDirectory := root
	t.Setenv("PATH", "")

	plan, err := manager.PlanModel("ServerSSHCredential", ModelGenerationOptions{PrimaryKeyColumn: "id"})
	if err != nil {
		t.Fatalf("plan model: %v", err)
	}

	wantPaths := []string{
		filepath.Join(root, "models", "server_ssh_credential.go"),
		filepath.Join(root, "models", "factories", "server_ssh_credential.go"),
		filepath.Join(root, "models", "model.go"),
	}
	gotPaths := make([]string, 0, len(plan.Files))
	for _, file := range plan.Files {
		gotPaths = append(gotPaths, file.Path)
		if file.NewContent == "" {
			t.Fatalf("planned file %s has empty content", file.Path)
		}
	}
	for _, want := range wantPaths {
		if !slices.Contains(gotPaths, want) {
			t.Fatalf("planned paths %#v do not contain %q", gotPaths, want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "models", "server_ssh_credential.go")); !os.IsNotExist(err) {
		t.Fatalf("planning wrote model file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "models", "factories")); !os.IsNotExist(err) {
		t.Fatalf("planning created factory directory: %v", err)
	}
	registryAfter, err := os.ReadFile(filepath.Join(root, "models", "model.go"))
	if err != nil {
		t.Fatalf("read registry after planning: %v", err)
	}
	if string(registryAfter) != string(originalRegistry) {
		t.Fatalf("planning changed registry\nbefore:\n%s\nafter:\n%s", originalRegistry, registryAfter)
	}
	workingDirectoryAfter, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory after planning: %v", err)
	}
	if workingDirectoryAfter != originalWorkingDirectory {
		t.Fatalf("planning changed working directory from %q to %q", originalWorkingDirectory, workingDirectoryAfter)
	}
}

func TestGenerateModelAppliesExactlyThePlannedContent(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	writeModelPlanningFixture(t, root)
	options := ModelGenerationOptions{PrimaryKeyColumn: "id", Mode: models.ModelModeCRUD}
	plan, err := manager.PlanModel("ServerSSHCredential", options)
	if err != nil {
		t.Fatalf("plan model: %v", err)
	}

	if err := manager.GenerateModelWithMode("ServerSSHCredential", "", false, "id", models.ModelModeCRUD); err != nil {
		t.Fatalf("generate model: %v", err)
	}
	for _, file := range plan.Files {
		content, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatalf("read applied file %s: %v", file.Path, err)
		}
		if string(content) != file.NewContent {
			t.Fatalf("applied content differs from plan for %s\nplanned:\n%s\napplied:\n%s", file.Path, file.NewContent, content)
		}
	}
}

func TestPlanModelFailureDoesNotApplyPartialChanges(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "models", "model.go"), []byte("package models\n\ntype (\n)\n\nvar (\n)\n"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	registryBefore, err := os.ReadFile(filepath.Join(root, "models", "model.go"))
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}

	if _, err := manager.PlanModel("Missing", ModelGenerationOptions{PrimaryKeyColumn: "id"}); err == nil {
		t.Fatal("expected planning failure for missing migration table")
	}
	if _, err := os.Stat(filepath.Join(root, "models", "missing.go")); !os.IsNotExist(err) {
		t.Fatalf("failed plan wrote model: %v", err)
	}
	registryAfter, err := os.ReadFile(filepath.Join(root, "models", "model.go"))
	if err != nil {
		t.Fatalf("read registry after failure: %v", err)
	}
	if string(registryAfter) != string(registryBefore) {
		t.Fatalf("failed plan changed registry\nbefore:\n%s\nafter:\n%s", registryBefore, registryAfter)
	}
}

func writeModelPlanningFixture(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "models", "model.go"), []byte("package models\n\ntype (\n)\n\nvar (\n)\n"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	migration := `-- +goose Up
CREATE TABLE server_ssh_credentials (
    id UUID PRIMARY KEY,
    url TEXT NOT NULL,
    cidr TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE server_ssh_credentials;
`
	if err := os.WriteFile(filepath.Join(root, "database", "migrations", "001_create_server_ssh_credentials.sql"), []byte(migration), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}
}

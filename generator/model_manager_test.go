package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/internal/cache"
)

func TestPlanModelRegistration(t *testing.T) {
	src := `package models

import "go.uber.org/fx"

var Module = fx.Module(
	"models",
	fx.Provide(
		NewUsers,
		NewTokens,
	),
)
`

	got, err := planModelRegistration("Server", src)
	if err != nil {
		t.Fatalf("planModelRegistration: %v", err)
	}
	if !strings.Contains(got, "NewServers,") {
		t.Errorf("expected NewServers inserted into fx.Provide; got:\n%s", got)
	}

	again, err := planModelRegistration("Server", got)
	if err != nil {
		t.Fatalf("planModelRegistration second call: %v", err)
	}
	if strings.Count(again, "NewServers") != 1 {
		t.Errorf("expected idempotent insert, got:\n%s", again)
	}
}

func setupModelManagerTest(t *testing.T) (*ModelManager, func()) {
	t.Helper()
	cache.ClearFileSystemCache()

	tmpDir := t.TempDir()

	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(
		filepath.Join(tmpDir, "go.mod"),
		[]byte(goModContent),
		0o644,
	); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	migrationsDir := filepath.Join(tmpDir, "migrations")
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

	plan, err := manager.PlanModel(
		"ServerSSHCredential",
		ModelGenerationOptions{PrimaryKeyColumn: "id"},
	)
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
	if _, err := os.Stat(
		filepath.Join(root, "models", "server_ssh_credential.go"),
	); !os.IsNotExist(
		err,
	) {
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
		t.Fatalf(
			"planning changed registry\nbefore:\n%s\nafter:\n%s",
			originalRegistry,
			registryAfter,
		)
	}
	workingDirectoryAfter, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory after planning: %v", err)
	}
	if workingDirectoryAfter != originalWorkingDirectory {
		t.Fatalf(
			"planning changed working directory from %q to %q",
			originalWorkingDirectory,
			workingDirectoryAfter,
		)
	}
}

func TestPlanModelIncludesExistingFactoryContent(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	writeModelPlanningFixture(t, root)
	factoryDir := filepath.Join(root, "models", "factories")
	if err := os.MkdirAll(factoryDir, 0o755); err != nil {
		t.Fatalf("create factory directory: %v", err)
	}
	factoryPath := filepath.Join(factoryDir, "server_ssh_credential.go")
	oldContent := "package factories\n"
	if err := os.WriteFile(factoryPath, []byte(oldContent), 0o644); err != nil {
		t.Fatalf("write existing factory: %v", err)
	}

	plan, err := manager.PlanModel(
		"ServerSSHCredential",
		ModelGenerationOptions{PrimaryKeyColumn: "id"},
	)
	if err != nil {
		t.Fatalf("plan model: %v", err)
	}
	for _, file := range plan.Files {
		if file.Path != factoryPath {
			continue
		}
		if !file.Exists {
			t.Fatal("existing factory was planned as a new file")
		}
		if file.OldContent != oldContent {
			t.Fatalf("factory old content = %q, want %q", file.OldContent, oldContent)
		}
		if file.NewContent == oldContent {
			t.Fatal("factory plan did not replace stale content")
		}
		return
	}
	t.Fatalf("plan did not include existing factory %q", factoryPath)
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

	if err := manager.GenerateModelWithMode(
		"ServerSSHCredential",
		"",
		false,
		"id",
		models.ModelModeCRUD,
	); err != nil {
		t.Fatalf("generate model: %v", err)
	}
	for _, file := range plan.Files {
		content, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatalf("read applied file %s: %v", file.Path, err)
		}
		if string(content) != file.NewContent {
			t.Fatalf(
				"applied content differs from plan for %s\nplanned:\n%s\napplied:\n%s",
				file.Path,
				file.NewContent,
				content,
			)
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
	if err := os.WriteFile(
		filepath.Join(root, "models", "model.go"),
		[]byte(modelNamespaceFixture),
		0o644,
	); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	registryBefore, err := os.ReadFile(filepath.Join(root, "models", "model.go"))
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}

	if _, err := manager.PlanModel(
		"Missing",
		ModelGenerationOptions{PrimaryKeyColumn: "id"},
	); err == nil {
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
		t.Fatalf(
			"failed plan changed registry\nbefore:\n%s\nafter:\n%s",
			registryBefore,
			registryAfter,
		)
	}
}

func writeModelPlanningFixture(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(root, "models", "model.go"),
		[]byte(modelNamespaceFixture),
		0o644,
	); err != nil {
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
	if err := os.WriteFile(
		filepath.Join(root, "migrations", "001_create_server_ssh_credentials.sql"),
		[]byte(migration),
		0o644,
	); err != nil {
		t.Fatalf("write migration: %v", err)
	}
}

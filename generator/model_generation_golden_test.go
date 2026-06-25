package generator

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/sebdah/goldie/v2"
)

func TestModelGenerationGoldens(t *testing.T) {
	g := goldie.New(t, goldie.WithFixtureDir(modelGenerationGoldenDir(t)))

	t.Run("initial_generation", func(t *testing.T) {
		manager := setupModelGoldenProject(t, "model_generation_initial")

		if err := manager.GenerateModel("Product", "", true, ""); err != nil {
			t.Fatalf("failed to generate model: %v", err)
		}

		content := readModelGoldenFile(t, manager, "Product")
		g.Assert(t, "product_initial", content)
	})

	t.Run("update_generation", func(t *testing.T) {
		manager := setupModelGoldenProject(t, "model_generation_initial")

		if err := manager.GenerateModel("Product", "", true, ""); err != nil {
			t.Fatalf("failed to generate initial model: %v", err)
		}

		manager.config.Database.MigrationDirs = []string{
			modelGenerationFixtureDir(t, "model_generation_updated"),
		}

		result, err := manager.UpdateModel("Product")
		if err != nil {
			t.Fatalf("failed to update model: %v", err)
		}
		if !result.HasChanges {
			t.Fatal("expected model update to report changes")
		}

		g.Assert(t, "product_updated", []byte(result.NewFileContent))

		diff, err := result.Diff()
		if err != nil {
			t.Fatalf("failed to diff model update: %v", err)
		}
		g.Assert(t, "product_update_diff", []byte(diff))
	})

	t.Run("custom_primary_key_generation", func(t *testing.T) {
		manager := setupModelGoldenProject(t, "model_generation_custom_pk")
		manager.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})

		if err := manager.GenerateModel("Order", "", true, ""); err != nil {
			t.Fatalf("failed to generate model: %v", err)
		}

		content := readModelGoldenFile(t, manager, "Order")
		g.Assert(t, "order_custom_pk", content)
	})

	t.Run("no_primary_key_generation", func(t *testing.T) {
		manager := setupModelGoldenProject(t, "model_generation_no_pk")
		manager.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})

		if err := manager.GenerateModel("AuditLog", "", true, ""); err != nil {
			t.Fatalf("failed to generate model: %v", err)
		}

		content := readModelGoldenFile(t, manager, "AuditLog")
		g.Assert(t, "audit_log_no_pk", content)
	})

	t.Run("no_primary_key_without_uuid_generation", func(t *testing.T) {
		manager := setupModelGoldenProject(t, "model_generation_no_pk_no_uuid")
		manager.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})

		if err := manager.GenerateModel("EventMetric", "", true, ""); err != nil {
			t.Fatalf("failed to generate model: %v", err)
		}

		content := readModelGoldenFile(t, manager, "EventMetric")
		g.Assert(t, "event_metric_no_pk_no_uuid", content)
	})
}

func setupModelGoldenProject(t *testing.T, migrationsFixture string) *ModelManager {
	t.Helper()

	cache.ClearFileSystemCache()
	t.Cleanup(cache.ClearFileSystemCache)

	projectDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})

	if err := os.WriteFile(
		filepath.Join(projectDir, "go.mod"),
		[]byte("module github.com/example/shop\n\ngo 1.26\n"),
		0o644,
	); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(projectDir, "models"), 0o755); err != nil {
		t.Fatalf("failed to create models directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "models", "model.go"), []byte(modelNamespaceFixture), 0o644); err != nil {
		t.Fatalf("failed to write models/model.go: %v", err)
	}

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to enter temp project: %v", err)
	}

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}

	coord.config.Database.MigrationDirs = []string{
		modelGenerationFixtureDir(t, migrationsFixture),
	}

	return coord.ModelManager
}

func readModelGoldenFile(t *testing.T, manager *ModelManager, resourceName string) []byte {
	t.Helper()

	content, err := os.ReadFile(BuildModelPath(manager.config.Paths.Models, resourceName))
	if err != nil {
		t.Fatalf("failed to read generated model: %v", err)
	}

	return content
}

func modelGenerationFixtureDir(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join(generatorPackageDir(t), "testdata", "migrations", name)
}

func modelGenerationGoldenDir(t *testing.T) string {
	t.Helper()

	return filepath.Join(generatorPackageDir(t), "testdata", "golden", "models")
}

func generatorPackageDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file")
	}
	return filepath.Dir(file)
}

const modelNamespaceFixture = `package models

type (
	token struct{}
	user  struct{}
)

var (
	Token token
	User  user
)
`

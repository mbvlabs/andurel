package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator"
	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestBuildModelPlanMutationReportUsesPlannedStrings(t *testing.T) {
	root := t.TempDir()
	plan := &generator.ModelGenerationPlan{Files: []generator.PlannedFile{
		{
			Path:       filepath.Join(root, "models", "model.go"),
			Exists:     true,
			OldContent: "package models\n",
			NewContent: "package models\n\nvar Product product\n",
		},
		{
			Path:       filepath.Join(root, "models", "product.go"),
			NewContent: "package models\n\ntype ProductEntity struct{}\n",
		},
	}}

	report := buildModelPlanMutationReport(root, "Product", plan, true)
	if !slices.Equal(report.FilesCreated, []string{"models/product.go"}) {
		t.Fatalf("created files = %#v", report.FilesCreated)
	}
	if !slices.Equal(report.FilesUpdated, []string{"models/model.go"}) {
		t.Fatalf("updated files = %#v", report.FilesUpdated)
	}
	for _, want := range []string{"diff --git a/models/model.go", "+var Product product", "diff --git a/models/product.go", "+type ProductEntity struct{}"} {
		if !strings.Contains(report.Diff, want) {
			t.Fatalf("planned diff missing %q:\n%s", want, report.Diff)
		}
	}
}

func TestGenerateModelDryRunReportsFactoryWithoutMutatingProject(t *testing.T) {
	resetCLITestSeams(t)

	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.26.5\n")
	writeCLITestFile(t, rootDir, "models/model.go", "package models\n\ntype (\n)\n\nvar (\n)\n")
	writeCLITestFile(t, rootDir, "database/migrations/000100_create_products.sql", `-- +goose Up
CREATE TABLE products (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE products;
`)

	modelRegistryPath := filepath.Join(rootDir, "models", "model.go")
	registryBefore, err := os.ReadFile(modelRegistryPath)
	if err != nil {
		t.Fatalf("read model registry before dry run: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("change to project root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
		cache.ClearFileSystemCache()
	})
	cache.ClearFileSystemCache()

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"generate", "model", "Product", "--dry-run", "--json", "--primary-key", "id"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate model dry run: %v", err)
	}

	var response struct {
		Data mutationReport `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode dry-run response: %v\n%s", err, stdout.String())
	}
	for _, want := range []string{"models/product.go", "models/factories/product.go"} {
		if !slices.Contains(response.Data.FilesCreated, want) {
			t.Fatalf("dry-run files_created missing %q: %#v", want, response.Data.FilesCreated)
		}
	}
	if !slices.Contains(response.Data.FilesUpdated, "models/model.go") {
		t.Fatalf("dry-run files_updated missing model registry: %#v", response.Data.FilesUpdated)
	}

	assertCLITestFileMissing(t, rootDir, "models/product.go")
	assertCLITestFileMissing(t, rootDir, "models/factories/product.go")
	registryAfter, err := os.ReadFile(modelRegistryPath)
	if err != nil {
		t.Fatalf("read model registry after dry run: %v", err)
	}
	if !bytes.Equal(registryBefore, registryAfter) {
		t.Fatalf("dry run changed models/model.go\nbefore:\n%s\nafter:\n%s", registryBefore, registryAfter)
	}
}

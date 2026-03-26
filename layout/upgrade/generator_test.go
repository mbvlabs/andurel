package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/layout"
)

func TestResolveModulePath(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	goModPath := filepath.Join(projectRoot, "go.mod")

	if err := os.WriteFile(goModPath, []byte("module github.com/example/myapp\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	modulePath, err := resolveModulePath(projectRoot)
	if err != nil {
		t.Fatalf("resolveModulePath returned error: %v", err)
	}

	if modulePath != "github.com/example/myapp" {
		t.Fatalf("resolveModulePath = %q, want %q", modulePath, "github.com/example/myapp")
	}
}

func TestBuildTemplateData_UsesModulePathInsteadOfProjectName(t *testing.T) {
	t.Parallel()

	generator := NewTemplateGenerator("v0.0.0")
	templateData := generator.buildTemplateData(layout.ScaffoldConfig{
		ProjectName:  "test-upgrade",
		Database:     "postgresql",
		CSSFramework: "tailwind",
	}, "github.com/example/correct-module")

	if templateData.ProjectName != "test-upgrade" {
		t.Fatalf("ProjectName = %q, want %q", templateData.ProjectName, "test-upgrade")
	}

	if templateData.ModuleName != "github.com/example/correct-module" {
		t.Fatalf(
			"ModuleName = %q, want %q",
			templateData.ModuleName,
			"github.com/example/correct-module",
		)
	}
}

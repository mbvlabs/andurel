package upgrade

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
		ProjectName: "test-upgrade",
		Database:    "postgresql",
	}, "github.com/example/correct-module", nil)

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

func TestGetFrameworkTemplates_ExcludesStandalonePackages(t *testing.T) {
	t.Parallel()

	for _, tmpl := range GetFrameworkTemplates(&layout.ScaffoldConfig{}) {
		if strings.HasPrefix(tmpl.TargetPath, "pkg/") {
			t.Fatalf("upgrade templates include standalone package source %s", tmpl.TargetPath)
		}
	}
}

func TestGetFrameworkTemplates_IncludesInertiaInternalPackageWhenConfigured(t *testing.T) {
	t.Parallel()

	for _, adapter := range []string{"vue", "react", "svelte"} {
		t.Run(adapter, func(t *testing.T) {
			t.Parallel()

			templates := GetFrameworkTemplates(&layout.ScaffoldConfig{Inertia: adapter})
			paths := make([]string, 0, len(templates))
			for _, tmpl := range templates {
				paths = append(paths, tmpl.TargetPath)
			}

			if !slices.Contains(paths, "internal/inertia/render.go") {
				t.Fatal("expected inertia render package file when inertia is configured")
			}
			if !slices.Contains(paths, "internal/inertia/vite.go") {
				t.Fatal("expected inertia vite package file when inertia is configured")
			}
		})
	}
}

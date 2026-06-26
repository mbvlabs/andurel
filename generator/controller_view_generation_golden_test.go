package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/sebdah/goldie/v2"
)

func TestControllerViewGenerationGoldens(t *testing.T) {
	g := goldie.New(t, goldie.WithFixtureDir(controllerViewGenerationGoldenDir(t)))

	scenarios := []struct {
		name           string
		withViews      bool
		initialActions []string
		actions        []string
	}{
		{name: "full_crud", withViews: true},
		{name: "single_action", withViews: true, actions: []string{"show"}},
		{name: "add_action", withViews: false, initialActions: []string{"show"}, actions: []string{"edit"}},
		{name: "add_view_action", withViews: true, initialActions: []string{"show"}, actions: []string{"edit"}},
		{name: "controller_only", withViews: false},
	}

	diModes := []string{"manual", "uberfx"}
	cssModes := []struct {
		name          string
		cssComponents bool
	}{
		{name: "bare"},
		{name: "css_components", cssComponents: true},
	}

	for _, scenario := range scenarios {
		for _, diMode := range diModes {
			for _, cssMode := range cssModes {
				testName := scenario.name + "_" + diMode + "_" + cssMode.name
				t.Run(testName, func(t *testing.T) {
					coord := setupControllerViewGoldenProject(t, diMode, cssMode.cssComponents)

					if len(scenario.initialActions) > 0 {
						if err := coord.GenerateControllerWithActions("Widget", "", scenario.withViews, scenario.initialActions, ""); err != nil {
							t.Fatalf("failed to generate initial controller/view: %v", err)
						}
					}

					if err := coord.GenerateControllerWithActions("Widget", "", scenario.withViews, scenario.actions, ""); err != nil {
						t.Fatalf("failed to generate controller/view: %v", err)
					}

					assertControllerViewArtifacts(t, g, testName, scenario.withViews, diMode)
				})
			}
		}
	}
}

func setupControllerViewGoldenProject(t *testing.T, diMode string, cssComponents bool) Coordinator {
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

	writeControllerViewFixtureFile(t, projectDir, "go.mod", "module testapp\n\ngo 1.26\n")
	writeControllerViewFixtureFile(t, projectDir, "models/model.go", modelNamespaceFixture)
	writeControllerViewFixtureFile(t, projectDir, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(projectDir, "bin", "templ"), 0o755); err != nil {
		t.Fatalf("failed to chmod fake templ binary: %v", err)
	}

	writeControllerViewFixtureFile(t, projectDir, "cmd/app/main.go", manualMainFixture)
	writeControllerViewFixtureFile(t, projectDir, "controllers/controller.go", fxControllerModuleFixture)

	lock := layout.NewAndurelLock("test")
	lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:  "testapp",
		Database:     "postgresql",
		CSSFramework: "tailwind",
		DIMode:       diMode,
	}
	if cssComponents {
		lock.ScaffoldConfig.Extensions = []string{"css-components"}
	}
	if err := lock.WriteLockFile(projectDir); err != nil {
		t.Fatalf("failed to write andurel.lock: %v", err)
	}

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to enter temp project: %v", err)
	}

	coord, err := NewCoordinator()
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	coord.config.Database.MigrationDirs = []string{
		modelGenerationFixtureDir(t, "controller_view_generation"),
	}
	coord.ModelManager.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})
	coord.ControllerManager.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})

	if err := coord.ModelManager.GenerateModel("Widget", "", true, ""); err != nil {
		t.Fatalf("failed to generate model prerequisite: %v", err)
	}

	return coord
}

func assertControllerViewArtifacts(t *testing.T, g *goldie.Goldie, fixtureDir string, withViews bool, diMode string) {
	t.Helper()

	paths := []string{
		"controllers/widgets.go",
		"router/routes/widgets.go",
	}
	if diMode == "manual" {
		paths = append(paths, "router/connect_widgets_routes.go", "cmd/app/main.go")
	} else {
		paths = append(paths, "controllers/controller.go")
	}
	if withViews {
		paths = append(paths, "views/widgets_resource.templ")
	} else {
		paths = append(paths, "!views/widgets_resource.templ")
	}

	for _, path := range paths {
		if path[0] == '!' {
			assertControllerViewGoldenFileMissing(t, path[1:])
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read generated artifact %s: %v", path, err)
		}
		g.Assert(t, filepath.Join(fixtureDir, path), content)
	}
}

func assertControllerViewGoldenFileMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s not to be generated", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("failed to stat %s: %v", path, err)
	}
}

func writeControllerViewFixtureFile(t *testing.T, root, relPath, content string) {
	t.Helper()

	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create fixture directory for %s: %v", relPath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture file %s: %v", relPath, err)
	}
}

func controllerViewGenerationGoldenDir(t *testing.T) string {
	t.Helper()

	return filepath.Join(generatorPackageDir(t), "testdata", "golden", "controller_views")
}

const manualMainFixture = `package main

import (
	"testapp/controllers"
	"testapp/router"
)

func setupControllers(db interface{}, r *router.Router) error {
	// andurel:controller-registration-point
	return nil
}
`

const fxControllerModuleFixture = `package controllers

import (
	"testapp/router"

	"go.uber.org/fx"
)

var constructors = fx.Provide(
)

var Module = fx.Module(
	"controllers",
	constructors,
)
`

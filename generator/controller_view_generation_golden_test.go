package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/sebdah/goldie/v2"
)

func TestControllerViewGenerationGoldens(t *testing.T) {
	g := goldie.New(t, goldie.WithFixtureDir(controllerViewGenerationGoldenDir(t)))

	scenarios := []struct {
		name           string
		initialActions []string
		actions        []string
	}{
		{name: "full_crud"},
		{name: "single_action", actions: []string{"show"}},
		{name: "add_action", initialActions: []string{"show"}, actions: []string{"edit"}},
		{name: "add_view_action", initialActions: []string{"show"}, actions: []string{"edit"}},
		{name: "controller_only"},
	}

	cssModes := []struct {
		name          string
		cssComponents bool
	}{
		{name: "bare"},
		{name: "css_components", cssComponents: true},
	}

	for _, scenario := range scenarios {
		for _, cssMode := range cssModes {
			testName := scenario.name + "_" + cssMode.name
			t.Run(testName, func(t *testing.T) {
				coord := setupControllerViewGoldenProject(t, cssMode.cssComponents)

				if len(scenario.initialActions) > 0 {
					if err := coord.GenerateControllerWithActions("Widget", "", "", scenario.initialActions, "", false); err != nil {
						t.Fatalf("failed to generate initial controller/view: %v", err)
					}
				}

				if err := coord.GenerateControllerWithActions("Widget", "", "", scenario.actions, "", false); err != nil {
					t.Fatalf("failed to generate controller/view: %v", err)
				}

				assertControllerViewArtifacts(t, g, testName)
			})
		}
	}
}

func TestControllerViewGenerationWithModelNameGolden(t *testing.T) {
	g := goldie.New(t, goldie.WithFixtureDir(controllerViewGenerationGoldenDir(t)))
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActionsForModel("Dashboard", "", "Widget", "", []string{"index", "show"}, "", false); err != nil {
		t.Fatalf("failed to generate controller/view with model name: %v", err)
	}

	assertControllerViewGoldenPaths(t, g, "model_name_bare", []string{
		"controllers/dashboards.go",
		"router/routes/dashboards.go",
		"controllers/controller.go",
	})
}

func TestControllerViewGenerationNamespacedController(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActions("Widget", "admin", "", nil, "", false); err != nil {
		t.Fatalf("failed to generate namespaced controller/view: %v", err)
	}

	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), `"testapp/controllers/admin"`)
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "admin.NewWidgets,")
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "func(r *router.Router, c admin.Widgets) error")
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "return c.RegisterRoutes(r)")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "func (w Widgets) RegisterRoutes(r *router.Router) error")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "routes.AdminWidgetIndex.Path()")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "routes.AdminWidgetShow.Path()")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "routes.AdminWidgetEdit.Path()")
}

func TestControllerViewGenerationRootAndNamespacedRegistrations(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActions("Widget", "admin", "", nil, "", false); err != nil {
		t.Fatalf("failed to generate namespaced controller/view: %v", err)
	}
	if err := coord.GenerateControllerWithActions("Widget", "", "", nil, "", false); err != nil {
		t.Fatalf("failed to generate root controller/view: %v", err)
	}

	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "\tadmin.NewWidgets,")
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "\tNewWidgets,")
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "func(r *router.Router, c admin.Widgets) error")
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "func(r *router.Router, c Widgets) error")
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "return c.RegisterRoutes(r)")
	assertGeneratedFileNotContains(t, filepath.Join("controllers", "controller.go"), "controllers.Widgets")
}

func TestControllerViewGenerationNamespacedModelName(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActionsForModel("Dashboard", "admin", "Widget", "", []string{"index", "show"}, "", false); err != nil {
		t.Fatalf("failed to generate namespaced controller/view with model name: %v", err)
	}

	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "dashboards.go"), "views.AdminDashboardIndex")
	assertGeneratedFileContains(t, filepath.Join("router", "routes", "admin_dashboards.go"), `"admin.dashboards.index"`)
	assertGeneratedFileContains(t, filepath.Join("controllers", "controller.go"), "admin.NewDashboards,")
	assertGeneratedFileContains(t, filepath.Join("views", "admin_dashboards_resource.templ"), "type AdminDashboardIndex struct")
}

func TestControllerViewGenerationNamespacedNoControllerGo(t *testing.T) {
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

	cache.ClearFileSystemCache()
	t.Cleanup(cache.ClearFileSystemCache)

	writeControllerViewFixtureFile(t, projectDir, "go.mod", "module testapp\n\ngo 1.26\n")
	writeControllerViewFixtureFile(t, projectDir, "models/model.go", modelNamespaceFixture)
	writeControllerViewFixtureFile(t, projectDir, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(projectDir, "bin", "templ"), 0o755); err != nil {
		t.Fatalf("failed to chmod fake templ binary: %v", err)
	}
	writeControllerViewFixtureFile(t, projectDir, "cmd/app/main.go", manualMainFixture)

	// Intentionally NOT writing controllers/controller.go —
	// this should be a non-fatal fallback, not an error.

	lock := layout.NewAndurelLock("test")
	lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "testapp",
		Database:    "postgresql",
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

	if err := coord.GenerateControllerWithActions("Widget", "admin", "", nil, "", false); err != nil {
		t.Fatalf("failed to generate namespaced controller/view without controller.go: %v", err)
	}

	// The controller file and view should still be generated
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), "package admin")
	assertGeneratedFileContains(t, filepath.Join("views", "admin_widgets_resource.templ"), "type AdminWidgetIndex struct")

	// controllers/controller.go should NOT exist (was never created, and generation
	// should not have created it — it only updates, it doesn't create)
	if _, err := os.Stat(filepath.Join(projectDir, "controllers", "controller.go")); err == nil {
		t.Fatal("expected controllers/controller.go to not exist after non-fatal fallback")
	}
}

func TestControllerViewGenerationNamespacedInertia(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActions("Widget", "admin", "", []string{"show"}, "vue", false); err != nil {
		t.Fatalf("failed to generate namespaced inertia controller/view: %v", err)
	}

	assertControllerViewGoldenFileMissing(t, filepath.Join("views", "admin_widgets_resource.templ"))
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Admin", "Widget", "Show.vue"), "<template>")
	assertGeneratedFileContains(t, filepath.Join("controllers", "admin", "widgets.go"), `return inertia.Page(etx, "Admin/Widget/Show"`)
}

func TestControllerViewGenerationGoldensInertiaProjectDefaultsToTempl(t *testing.T) {
	coord := setupControllerViewGoldenProjectWithInertia(t, false, "vue")

	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"index", "show"}, "", false); err != nil {
		t.Fatalf("failed to generate controller/view: %v", err)
	}

	assertGeneratedFileContains(t, "views/widgets_resource.templ", "type WidgetIndex struct")
	assertControllerViewGoldenFileMissing(t, filepath.Join("resources", "js", "Pages", "Widget", "Index.vue"))
	assertGeneratedFileContains(t, "controllers/widgets.go", "testapp/internal/hypermedia")
	assertGeneratedFileNotContains(t, "controllers/widgets.go", "testapp/internal/inertia")
}

func TestControllerViewGenerationGoldensInertiaFlagStillGeneratesInertia(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"index", "show"}, "vue", false); err != nil {
		t.Fatalf("failed to generate controller/view: %v", err)
	}

	assertControllerViewGoldenFileMissing(t, "views/widgets_resource.templ")
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Widget", "Index.vue"), "<template>")
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Widget", "Show.vue"), "<template>")
	assertGeneratedFileContains(t, "controllers/widgets.go", "testapp/internal/inertia")
	assertGeneratedFileNotContains(t, "controllers/widgets.go", "testapp/internal/hypermedia")
}

func TestControllerViewGenerationGoldensReactInertiaFlagGeneratesReactPages(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"index", "show"}, "react", false); err != nil {
		t.Fatalf("failed to generate controller/view: %v", err)
	}

	assertControllerViewGoldenFileMissing(t, "views/widgets_resource.templ")
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Widget", "Index.tsx"), "@inertiajs/react")
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Widget", "Show.tsx"), "export default function Show")
	assertGeneratedFileContains(t, "controllers/widgets.go", "testapp/internal/inertia")
	assertGeneratedFileNotContains(t, "controllers/widgets.go", "testapp/internal/hypermedia")
	assertGeneratedFileContains(t, "controllers/widgets.go", "return inertia.Page(etx, \"Widget/Index\"")
}

func TestControllerViewGenerationGoldensSingleVueActionGeneratesInertiaController(t *testing.T) {
	coord := setupControllerViewGoldenProject(t, false)

	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"show"}, "vue", false); err != nil {
		t.Fatalf("failed to generate controller/view: %v", err)
	}

	assertControllerViewGoldenFileMissing(t, "views/widgets_resource.templ")
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Widget", "Show.vue"), "<template>")
	assertGeneratedFileContains(t, "controllers/widgets.go", "testapp/internal/inertia")
	assertGeneratedFileNotContains(t, "controllers/widgets.go", "testapp/internal/hypermedia")
	assertGeneratedFileContains(t, "controllers/widgets.go", "return inertia.Page(etx, \"Widget/Show\"")
}

func TestControllerViewGenerationGoldensVueActionDoesNotInheritTemplViewActions(t *testing.T) {
	coord := setupControllerViewGoldenProjectWithInertia(t, false, "vue")

	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"index"}, "", false); err != nil {
		t.Fatalf("failed to generate templ controller/view: %v", err)
	}
	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"show"}, "vue", false); err != nil {
		t.Fatalf("failed to generate vue controller/view: %v", err)
	}

	assertGeneratedFileContains(t, "views/widgets_resource.templ", "type WidgetIndex struct")
	assertGeneratedFileContains(t, filepath.Join("resources", "js", "Pages", "Widget", "Show.vue"), "<template>")
	assertControllerViewGoldenFileMissing(t, filepath.Join("resources", "js", "Pages", "Widget", "Index.vue"))
	assertGeneratedFileContains(t, "controllers/widgets.go", "testapp/internal/hypermedia")
	assertGeneratedFileContains(t, "controllers/widgets.go", "testapp/internal/inertia")
	assertGeneratedFileContains(t, "controllers/widgets.go", "return hypermedia.RenderPage(etx, views.WidgetIndex")
	assertGeneratedFileContains(t, "controllers/widgets.go", "return inertia.Page(etx, \"Widget/Show\"")
}

func TestControllerViewGenerationGoldensVueActionUpdatesRegisterRoutes(t *testing.T) {
	coord := setupControllerViewGoldenProjectWithInertia(t, false, "vue")

	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"index"}, "", false); err != nil {
		t.Fatalf("failed to generate templ controller/view: %v", err)
	}
	if err := coord.GenerateControllerWithActions("Widget", "", "", []string{"show"}, "vue", false); err != nil {
		t.Fatalf("failed to generate vue controller/view: %v", err)
	}

	assertGeneratedFileContains(t, "controllers/widgets.go", "routes.WidgetIndex.Path()")
	assertGeneratedFileContains(t, "controllers/widgets.go", "routes.WidgetShow.Path()")
	assertGeneratedFileContains(t, "controllers/widgets.go", "Handler: w.Index")
	assertGeneratedFileContains(t, "controllers/widgets.go", "Handler: w.Show")
	assertGeneratedFileContains(t, "controllers/widgets.go", "return hypermedia.RenderPage(etx, views.WidgetIndex")
	assertGeneratedFileContains(t, "controllers/widgets.go", "return inertia.Page(etx, \"Widget/Show\"")
}

func setupControllerViewGoldenProject(t *testing.T, cssComponents bool) Coordinator {
	return setupControllerViewGoldenProjectWithInertia(t, cssComponents, "")
}

func setupControllerViewGoldenProjectWithInertia(t *testing.T, cssComponents bool, inertia string) Coordinator {
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
		ProjectName: "testapp",
		Database:    "postgresql",
		Inertia:     inertia,
	}
	if cssComponents {
		lock.AddExtension("css-components", "test-applied-at")
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

func assertControllerViewArtifacts(t *testing.T, g *goldie.Goldie, fixtureDir string) {
	t.Helper()

	paths := []string{
		"controllers/widgets.go",
		"router/routes/widgets.go",
		"controllers/controller.go",
	}
	paths = append(paths, "views/widgets_resource.templ")

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

func assertControllerViewGoldenPaths(t *testing.T, g *goldie.Goldie, fixtureDir string, paths []string) {
	t.Helper()

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

func assertGeneratedFileContains(t *testing.T, path, want string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated artifact %s: %v", path, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("expected %s to contain %q:\n%s", path, want, string(content))
	}
}

func assertGeneratedFileNotContains(t *testing.T, path, unwanted string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated artifact %s: %v", path, err)
	}
	if strings.Contains(string(content), unwanted) {
		t.Fatalf("expected %s not to contain %q:\n%s", path, unwanted, string(content))
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

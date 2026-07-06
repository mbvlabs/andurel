package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout"
)

func TestNoArgGeneratorCommandsShowHelpWithoutCallingGenerators(t *testing.T) {
	tests := [][]string{
		{"generate", "model"},
		{"generate", "controller"},
		{"generate", "scaffold"},
		{"generate", "job"},
		{"generate", "email"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			resetCLITestSeams(t)
			fake := installFakeGenerator(t)

			result := executeCLITest(t, args...)
			if result.err != nil {
				t.Fatalf("expected help without error, got %v", result.err)
			}
			if len(fake.modelCalls) != 0 || len(fake.modelWithPKCalls) != 0 || len(fake.scaffoldCalls) != 0 || len(fake.controllerCalls) != 0 {
				t.Fatalf("expected no generator calls, got %#v", fake)
			}
		})
	}
}

func TestGenerateCommandsRejectTooManyArgs(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{args: []string{"generate", "model", "Post", "Extra"}, message: "model takes exactly 1 argument"},
		{args: []string{"generate", "scaffold", "Post", "Extra"}, message: "scaffold takes exactly 1 argument"},
		{args: []string{"generate", "job", "SendEmail", "Extra"}, message: "job takes exactly 1 argument"},
		{args: []string{"generate", "email", "WelcomeEmail", "Extra"}, message: "email takes exactly 1 argument"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			result := runCLITest(t, tt.args...)
			if result.err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(result.err.Error(), tt.message) {
				t.Fatalf("expected error containing %q, got %v", tt.message, result.err)
			}
		})
	}
}

func TestGenerateModelMapsFlagsToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "model", "Widget", "--skip-factory", "--table-name", "inventory_widgets")
	if result.err != nil {
		t.Fatalf("generate model failed: %v", result.err)
	}

	want := []modelCall{{
		name:        "Widget",
		tableName:   "inventory_widgets",
		skipFactory: true,
	}}
	if !reflect.DeepEqual(fake.modelCalls, want) {
		t.Fatalf("model calls: expected %#v, got %#v", want, fake.modelCalls)
	}
	if len(fake.modelWithPKCalls) != 0 {
		t.Fatalf("expected GenerateModelWithPK not to be called, got %#v", fake.modelWithPKCalls)
	}
}

func TestGenerateModelMapsPrimaryKeyToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "model", "Warehouse", "--skip-factory", "--table-name", "warehouses", "--primary-key", "code")
	if result.err != nil {
		t.Fatalf("generate model failed: %v", result.err)
	}

	want := []modelWithPKCall{{
		name:        "Warehouse",
		tableName:   "warehouses",
		skipFactory: true,
		primaryKey:  "code",
	}}
	if !reflect.DeepEqual(fake.modelWithPKCalls, want) {
		t.Fatalf("model with pk calls: expected %#v, got %#v", want, fake.modelWithPKCalls)
	}
	if len(fake.modelCalls) != 0 {
		t.Fatalf("expected GenerateModel not to be called, got %#v", fake.modelCalls)
	}
}

func TestGenerateModelUpdateMapsYesFlag(t *testing.T) {
	resetCLITestSeams(t)
	var gotName string
	var gotAutoApply bool
	runModelUpdateFunc = func(resourceName string, autoApply bool) error {
		gotName = resourceName
		gotAutoApply = autoApply
		return nil
	}

	result := executeCLITest(t, "generate", "model", "Widget", "--update", "--yes")
	if result.err != nil {
		t.Fatalf("generate model update failed: %v", result.err)
	}
	if gotName != "Widget" || !gotAutoApply {
		t.Fatalf("expected update Widget autoApply=true, got name=%q autoApply=%v", gotName, gotAutoApply)
	}
}

func TestGenerateModelRunsFromProjectRoot(t *testing.T) {
	resetCLITestSeams(t)

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	nestedDir := filepath.Join(rootDir, "internal", "feature")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(nestedDir); err != nil {
		t.Fatalf("chdir nested: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}

	fake := installFakeGenerator(t)
	var gotWD string
	fake.onGenerateModel = func() {
		gotWD, _ = os.Getwd()
	}

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"generate", "model", "Widget"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate model failed: %v", err)
	}
	if gotWD != rootDir {
		t.Fatalf("expected generator to run in project root %q, got %q", rootDir, gotWD)
	}
}

func TestGenerateScaffoldMapsFlagsToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "scaffold", "Project", "--skip-factory", "--table-name", "work_projects", "--primary-key", "slug", "--inertia")
	if result.err != nil {
		t.Fatalf("generate scaffold failed: %v", result.err)
	}

	want := []scaffoldCall{{
		name:        "Project",
		namespace:   "",
		tableName:   "work_projects",
		skipFactory: true,
		primaryKey:  "slug",
		inertia:     "",
	}}
	if !reflect.DeepEqual(fake.scaffoldCalls, want) {
		t.Fatalf("scaffold calls: expected %#v, got %#v", want, fake.scaffoldCalls)
	}
}

func TestGenerateScaffoldMapsNamespaceToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "scaffold", "admin/Widget")
	if result.err != nil {
		t.Fatalf("generate scaffold failed: %v", result.err)
	}

	want := []scaffoldCall{{
		name:      "Widget",
		namespace: "admin",
	}}
	if !reflect.DeepEqual(fake.scaffoldCalls, want) {
		t.Fatalf("scaffold calls: expected %#v, got %#v", want, fake.scaffoldCalls)
	}
}

func TestGenerateScaffoldRejectsInvalidNamespaceBeforeGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "scaffold", "admin/reports/Widget")
	if result.err == nil {
		t.Fatalf("expected invalid namespace error")
	}
	if len(fake.scaffoldCalls) != 0 {
		t.Fatalf("expected no scaffold calls, got %#v", fake.scaffoldCalls)
	}
}

func TestGenerateControllerMapsActionsAndVue(t *testing.T) {
	resetCLITestSeams(t)
	var got controllerCall
	generateControllerWithActionsFunc = func(name, modelName string, actions []string, inertia string, isAPI bool) error {
		got = controllerCall{
			name:      name,
			modelName: modelName,
			actions:   append([]string(nil), actions...),
			inertia:   inertia,
		}
		return nil
	}

	result := executeCLITest(t, "generate", "controller", "Widget", "index", "export", "--inertia")
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := controllerCall{
		name:      "Widget",
		modelName: "",
		actions:   []string{"index", "export"},
		inertia:   "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("controller call: expected %#v, got %#v", want, got)
	}
}

func TestGenerateControllerMapsNamespaceToGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "controller", "admin/Widget", "index")
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := []controllerCall{{
		name:      "Widget",
		namespace: "admin",
		modelName: "Widget",
		actions:   []string{"index"},
	}}
	if !reflect.DeepEqual(fake.controllerCalls, want) {
		t.Fatalf("controller calls: expected %#v, got %#v", want, fake.controllerCalls)
	}
}

func TestGenerateControllerRejectsInvalidNamespaceBeforeGenerator(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)

	result := executeCLITest(t, "generate", "controller", "admin/reports/Widget", "index")
	if result.err == nil {
		t.Fatalf("expected invalid namespace error")
	}
	if len(fake.controllerCalls) != 0 {
		t.Fatalf("expected no controller calls, got %#v", fake.controllerCalls)
	}
}

func TestGenerateControllerMapsModelName(t *testing.T) {
	resetCLITestSeams(t)
	var got controllerCall
	generateControllerWithActionsFunc = func(name, modelName string, actions []string, inertia string, isAPI bool) error {
		got = controllerCall{
			name:      name,
			modelName: modelName,
			actions:   append([]string(nil), actions...),
			inertia:   inertia,
		}
		return nil
	}

	result := executeCLITest(t, "generate", "controller", "Dashboard", "index", "--model-name", "User")
	if result.err != nil {
		t.Fatalf("generate controller failed: %v", result.err)
	}

	want := controllerCall{
		name:      "Dashboard",
		modelName: "User",
		actions:   []string{"index"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("controller call: expected %#v, got %#v", want, got)
	}
}

func TestGenerateControllerCustomActionCreatesRouteWithoutModel(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeCLITestFile(t, rootDir, "controllers/controller.go", `package controllers

import (
	"example.com/app/router"

	"go.uber.org/fx"
)

var constructors = fx.Provide(
)

var Module = fx.Module(
	"controllers",
	constructors,
)
`)

	writeGenerateFileTestLock(t, rootDir)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := generateControllerWithActions("Dashboard", "", []string{"overview"}, "", false); err != nil {
		t.Fatalf("generate custom controller action: %v", err)
	}

	assertCLITestFileContains(t, rootDir, "controllers/dashboards.go", "func (d Dashboards) Overview(etx *echo.Context) error")
	assertCLITestFileContains(t, rootDir, "views/dashboards_resource.templ", "templ DashboardOverview()")
	assertCLITestFileContains(t, rootDir, "router/routes/dashboards.go", "var DashboardOverview = routing.NewSimpleRoute")
	assertCLITestFileContains(t, rootDir, "router/routes/dashboards.go", "\"dashboards.overview\"")
	assertCLITestFileContains(t, rootDir, "controllers/controller.go", "NewDashboards,")
}

func TestGenerateControllerNamespacedCustomActionCreatesNamespacedArtifacts(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeCLITestFile(t, rootDir, "controllers/controller.go", `package controllers

import (
	"example.com/app/router"

	"go.uber.org/fx"
)

var constructors = fx.Provide(
)

var Module = fx.Module(
	"controllers",
	constructors,
)
`)

	writeGenerateFileTestLock(t, rootDir)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := generateControllerWithActions("admin/Widget", "", []string{"export"}, "", false); err != nil {
		t.Fatalf("generate namespaced custom controller action: %v", err)
	}

	assertCLITestFileContains(t, rootDir, filepath.Join("controllers", "admin", "widgets.go"), "package admin")
	assertCLITestFileContains(t, rootDir, filepath.Join("controllers", "admin", "widgets.go"), "func (w Widgets) Export(etx *echo.Context) error")
	assertCLITestFileContains(t, rootDir, filepath.Join("controllers", "admin", "widgets.go"), "views.AdminWidgetExport()")
	assertCLITestFileContains(t, rootDir, filepath.Join("views", "admin_widgets_resource.templ"), "templ AdminWidgetExport()")
	assertCLITestFileContains(t, rootDir, filepath.Join("router", "routes", "admin_widgets.go"), "const AdminWidgetPrefix =")
	assertCLITestFileContains(t, rootDir, filepath.Join("router", "routes", "admin_widgets.go"), "var AdminWidgetExport = routing.NewSimpleRoute")
	assertCLITestFileContains(t, rootDir, filepath.Join("router", "routes", "admin_widgets.go"), `"admin.widgets.export"`)
	assertCLITestFileContains(t, rootDir, filepath.Join("controllers", "controller.go"), `"example.com/app/controllers/admin"`)
	assertCLITestFileContains(t, rootDir, filepath.Join("controllers", "controller.go"), "admin.NewWidgets,")
}

func TestGenerateControllerCustomActionInertiaProjectDefaultsToTemplAndInertiaFlag(t *testing.T) {
	tests := []struct {
		name               string
		inertia            string
		wantController     string
		unwantedController string
		wantView           string
		unwantedView       string
	}{
		{
			name:               "default templ",
			wantController:     "example.com/app/internal/hypermedia",
			unwantedController: "example.com/app/internal/inertia",
			wantView:           "views/dashboards_resource.templ",
			unwantedView:       filepath.Join("resources", "js", "Pages", "Dashboard", "Overview.vue"),
		},
		{
			name:               "explicit vue",
			inertia:            "vue",
			wantController:     "example.com/app/internal/inertia",
			unwantedController: "example.com/app/internal/hypermedia",
			wantView:           filepath.Join("resources", "js", "Pages", "Dashboard", "Overview.vue"),
			unwantedView:       "views/dashboards_resource.templ",
		},
		{
			name:               "explicit react",
			inertia:            "react",
			wantController:     "example.com/app/internal/inertia",
			unwantedController: "example.com/app/internal/hypermedia",
			wantView:           filepath.Join("resources", "js", "Pages", "Dashboard", "Overview.tsx"),
			unwantedView:       "views/dashboards_resource.templ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLITestSeams(t)
			rootDir := t.TempDir()
			writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.26\n")

			lock := layout.NewAndurelLock("test")
			lock.ScaffoldConfig = &layout.ScaffoldConfig{
				ProjectName: "app",
				Database:    "postgresql",
				Inertia:     "vue",
			}
			if err := lock.WriteLockFile(rootDir); err != nil {
				t.Fatalf("write andurel.lock: %v", err)
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("getwd: %v", err)
			}
			if err := os.Chdir(rootDir); err != nil {
				t.Fatalf("chdir temp project: %v", err)
			}
			t.Cleanup(func() {
				_ = os.Chdir(originalWD)
			})

			if err := generateControllerWithActions("Dashboard", "", []string{"overview"}, tt.inertia, false); err != nil {
				t.Fatalf("generate custom controller action: %v", err)
			}

			assertCLITestFileContains(t, rootDir, "controllers/dashboards.go", tt.wantController)
			assertCLITestFileNotContains(t, rootDir, "controllers/dashboards.go", tt.unwantedController)
			assertCLITestFileExists(t, rootDir, tt.wantView)
			assertCLITestFileMissing(t, rootDir, tt.unwantedView)
		})
	}
}

func TestGenerateControllerSingleCRUDActionVueGeneratesInertiaController(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	setupProjectInquiryCLITestProject(t, rootDir)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := generateControllerWithActions("ProjectInquiry", "", []string{"show"}, "vue", false); err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileContains(t, rootDir, "controllers/project_inquiries.go", "example.com/app/internal/inertia")
	assertCLITestFileContains(t, rootDir, "controllers/project_inquiries.go", `return inertia.Page(etx, "ProjectInquiry/Show"`)
	assertCLITestFileNotContains(t, rootDir, "controllers/project_inquiries.go", "example.com/app/internal/hypermedia")
	assertCLITestFileExists(t, rootDir, filepath.Join("resources", "js", "Pages", "ProjectInquiry", "Show.vue"))
	assertCLITestFileMissing(t, rootDir, "views/project_inquiries_resource.templ")
}

func TestGenerateControllerSingleCRUDActionReactGeneratesInertiaController(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	setupProjectInquiryCLITestProject(t, rootDir)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := generateControllerWithActions("ProjectInquiry", "", []string{"show"}, "react", false); err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileContains(t, rootDir, "controllers/project_inquiries.go", "example.com/app/internal/inertia")
	assertCLITestFileContains(t, rootDir, "controllers/project_inquiries.go", `return inertia.Page(etx, "ProjectInquiry/Show"`)
	assertCLITestFileNotContains(t, rootDir, "controllers/project_inquiries.go", "example.com/app/internal/hypermedia")
	assertCLITestFileExists(t, rootDir, filepath.Join("resources", "js", "Pages", "ProjectInquiry", "Show.tsx"))
	assertCLITestFileMissing(t, rootDir, "views/project_inquiries_resource.templ")
}

func setupProjectInquiryCLITestProject(t *testing.T, rootDir string) {
	t.Helper()

	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeCLITestFile(t, rootDir, "database/migrations/000100_create_project_inquiries.sql", `-- +goose Up
CREATE TABLE project_inquiries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE project_inquiries;
`)
	writeCLITestFile(t, rootDir, "models/project_inquiry.go", "package models\n")
	writeCLITestFile(t, rootDir, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(rootDir, "bin", "templ"), 0o755); err != nil {
		t.Fatalf("chmod fake templ: %v", err)
	}

	lock := layout.NewAndurelLock("test")
	lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
		Database:    "postgresql",
		Inertia:     "vue",
	}
	if err := lock.WriteLockFile(rootDir); err != nil {
		t.Fatalf("write andurel.lock: %v", err)
	}
}

func TestGenerateControllerSingleCRUDActionInertiaProjectDefaultsToTemplController(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	setupProjectInquiryCLITestProject(t, rootDir)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := generateControllerWithActions("ProjectInquiry", "", []string{"index"}, "", false); err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileContains(t, rootDir, "controllers/project_inquiries.go", "example.com/app/internal/hypermedia")
	assertCLITestFileContains(t, rootDir, "controllers/project_inquiries.go", "return hypermedia.RenderPage")
	assertCLITestFileNotContains(t, rootDir, "controllers/project_inquiries.go", "example.com/app/internal/inertia")
	assertCLITestFileExists(t, rootDir, "views/project_inquiries_resource.templ")
	assertCLITestFileContains(t, rootDir, "views/project_inquiries_resource.templ", "Items []models.ProjectInquiryEntity")
	assertCLITestFileNotContains(t, rootDir, "views/project_inquiries_resource.templ", "ProjectinquiryEntity")
	assertCLITestFileMissing(t, rootDir, filepath.Join("resources", "js", "Pages", "ProjectInquiry", "Index.vue"))
}

func TestGenerateControllerRejectsModelNameForCustomOnly(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.26\n")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	err = generateControllerWithActions("Dashboard", "User", []string{"overview"}, "", false)
	if err == nil || !strings.Contains(err.Error(), "--model-name requires") {
		t.Fatalf("expected --model-name custom-only error, got %v", err)
	}
}

func TestControllerActionClassification(t *testing.T) {
	actions := []string{"index", "show", "export", "INDEX", "archive"}

	if got := crudControllerActions(actions); !reflect.DeepEqual(got, []string{"index", "show"}) {
		t.Fatalf("crud actions: expected [index show], got %v", got)
	}
	if got := nonCRUDControllerActions(actions); !reflect.DeepEqual(got, []string{"export", "archive"}) {
		t.Fatalf("custom actions: expected [export archive], got %v", got)
	}
}

func writeCLITestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func assertCLITestFileContains(t *testing.T, root, relPath, want string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("expected %s to contain %q:\n%s", relPath, want, string(content))
	}
}

func assertCLITestFileNotContains(t *testing.T, root, relPath, unwanted string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	if strings.Contains(string(content), unwanted) {
		t.Fatalf("expected %s not to contain %q:\n%s", relPath, unwanted, string(content))
	}
}

func assertCLITestFileExists(t *testing.T, root, relPath string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, relPath)); err != nil {
		t.Fatalf("expected %s to exist: %v", relPath, err)
	}
}

func assertCLITestFileMissing(t *testing.T, root, relPath string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, relPath)); err == nil {
		t.Fatalf("expected %s to be missing", relPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", relPath, err)
	}
}

func TestGenerateViewCallsTemplGenerate(t *testing.T) {
	resetCLITestSeams(t)
	var got []string
	runTemplFunc = func(args ...string) error {
		got = append([]string(nil), args...)
		return nil
	}

	result := executeCLITest(t, "generate", "view")
	if result.err != nil {
		t.Fatalf("generate view failed: %v", result.err)
	}
	if !reflect.DeepEqual(got, []string{"generate"}) {
		t.Fatalf("templ args: expected [generate], got %v", got)
	}
}

func TestFmtCommandMapsFlags(t *testing.T) {
	resetCLITestSeams(t)
	var gotRoot string
	var gotCheck, gotSkipTempl, gotSkipGo bool
	runFmtFunc = func(rootDir string, checkMode, skipTempl, skipGo bool) error {
		gotRoot = rootDir
		gotCheck = checkMode
		gotSkipTempl = skipTempl
		gotSkipGo = skipGo
		return nil
	}

	result := executeCLITest(t, "fmt", "--check", "--skip-templ", "--skip-go")
	if result.err != nil {
		t.Fatalf("fmt failed: %v", result.err)
	}
	if gotRoot == "" || !gotCheck || !gotSkipTempl || !gotSkipGo {
		t.Fatalf("unexpected fmt call root=%q check=%v skipTempl=%v skipGo=%v", gotRoot, gotCheck, gotSkipTempl, gotSkipGo)
	}
}

func TestRunFmtHonorsSkipFlags(t *testing.T) {
	resetCLITestSeams(t)
	var calls []string
	runGoFmtFunc = func(rootDir string, checkMode bool) error {
		calls = append(calls, "gofmt")
		return nil
	}
	runGolinesFunc = func(rootDir string, checkMode bool) error {
		calls = append(calls, "golines")
		return nil
	}
	runTemplFmtFunc = func(rootDir string, checkMode bool) error {
		calls = append(calls, "templ")
		return nil
	}

	if err := runFmt("project", true, true, false); err != nil {
		t.Fatalf("runFmt failed: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"gofmt", "golines"}) {
		t.Fatalf("skip templ calls: expected [gofmt golines], got %v", calls)
	}

	calls = nil
	if err := runFmt("project", true, false, true); err != nil {
		t.Fatalf("runFmt failed: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"templ"}) {
		t.Fatalf("skip go calls: expected [templ], got %v", calls)
	}
}

func TestRunFmtCheckModeReturnsFormattingError(t *testing.T) {
	resetCLITestSeams(t)
	runGoFmtFunc = func(rootDir string, checkMode bool) error {
		if !checkMode {
			t.Fatalf("expected checkMode=true")
		}
		return errors.New("needs formatting")
	}
	runGolinesFunc = func(rootDir string, checkMode bool) error { return nil }
	runTemplFmtFunc = func(rootDir string, checkMode bool) error { return nil }

	err := runFmt("project", true, true, false)
	if err == nil || !strings.Contains(err.Error(), "some files need formatting") {
		t.Fatalf("expected check formatting error, got %v", err)
	}
}

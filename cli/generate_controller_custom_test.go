package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout"
)

func TestGenerateControllerCustomActionCreatesRouteWithoutModel(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeEmptyControllersModule(t, rootDir)
	chdirCLITestRoot(t, rootDir)

	if err := generateControllerWithActions(
		"Dashboard",
		"",
		[]string{"overview"},
		"",
		false,
	); err != nil {
		t.Fatalf("generate custom controller action: %v", err)
	}

	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/dashboards.go",
		"func (d Dashboards) Overview(etx *echo.Context) error",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"views/dashboards_resource.templ",
		"templ DashboardOverview()",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"router/routes/dashboards.go",
		"var DashboardOverview = routing.NewSimpleRoute",
	)
	assertCLITestFileContains(t, rootDir, "router/routes/dashboards.go", `"dashboards.overview"`)
	assertCLITestFileContains(t, rootDir, "controllers/controller.go", "NewDashboards,")
}

func TestGenerateControllerNamespacedCustomActionCreatesNamespacedArtifacts(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeEmptyControllersModule(t, rootDir)
	writeGenerateFileTestLock(t, rootDir)
	chdirCLITestRoot(t, rootDir)

	if err := generateControllerWithActions(
		"admin/Widget",
		"",
		[]string{"export"},
		"",
		false,
	); err != nil {
		t.Fatalf("generate namespaced custom controller action: %v", err)
	}

	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "admin", "widgets.go"),
		"package admin",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "admin", "widgets.go"),
		"func (w Widgets) Export(etx *echo.Context) error",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "admin", "widgets.go"),
		"views.AdminWidgetExport()",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("views", "admin_widgets_resource.templ"),
		"templ AdminWidgetExport()",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("router", "routes", "admin_widgets.go"),
		"const AdminWidgetPrefix =",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("router", "routes", "admin_widgets.go"),
		"var AdminWidgetExport = routing.NewSimpleRoute",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("router", "routes", "admin_widgets.go"),
		`"admin.widgets.export"`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "controller.go"),
		`"example.com/app/controllers/admin"`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "controller.go"),
		"admin.NewWidgets,",
	)
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
			wantController:     "github.com/mbvlabs/andurel/pkg/hypermedia",
			unwantedController: "github.com/mbvlabs/andurel/pkg/inertia",
			wantView:           "views/dashboards_resource.templ",
			unwantedView: filepath.Join(
				"resources",
				"js",
				"Pages",
				"Dashboard",
				"Overview.vue",
			),
		},
		{
			name:               "explicit vue",
			inertia:            "vue",
			wantController:     "github.com/mbvlabs/andurel/pkg/inertia",
			unwantedController: "github.com/mbvlabs/andurel/pkg/hypermedia",
			wantView: filepath.Join(
				"resources",
				"js",
				"Pages",
				"Dashboard",
				"Overview.vue",
			),
			unwantedView: "views/dashboards_resource.templ",
		},
		{
			name:               "explicit react",
			inertia:            "react",
			wantController:     "github.com/mbvlabs/andurel/pkg/inertia",
			unwantedController: "github.com/mbvlabs/andurel/pkg/hypermedia",
			wantView: filepath.Join(
				"resources",
				"js",
				"Pages",
				"Dashboard",
				"Overview.tsx",
			),
			unwantedView: "views/dashboards_resource.templ",
		},
		{
			name:               "explicit svelte",
			inertia:            "svelte",
			wantController:     "github.com/mbvlabs/andurel/pkg/inertia",
			unwantedController: "github.com/mbvlabs/andurel/pkg/hypermedia",
			wantView: filepath.Join(
				"resources",
				"js",
				"Pages",
				"Dashboard",
				"Overview.svelte",
			),
			unwantedView: "views/dashboards_resource.templ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLITestSeams(t)
			rootDir := t.TempDir()
			writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")

			lock := layout.NewAndurelLock("test")
			lock.ScaffoldConfig = &layout.ScaffoldConfig{
				ProjectName: "app",
				Inertia:     "vue",
			}
			if err := lock.WriteLockFile(rootDir); err != nil {
				t.Fatalf("write andurel.lock: %v", err)
			}

			chdirCLITestRoot(t, rootDir)

			if err := generateControllerWithActions(
				"Dashboard",
				"",
				[]string{"overview"},
				tt.inertia,
				false,
			); err != nil {
				t.Fatalf("generate custom controller action: %v", err)
			}

			assertCLITestFileContains(t, rootDir, "controllers/dashboards.go", tt.wantController)
			assertCLITestFileNotContains(
				t,
				rootDir,
				"controllers/dashboards.go",
				tt.unwantedController,
			)
			if tt.inertia != "" {
				assertCLITestFileContains(
					t,
					rootDir,
					"controllers/dashboards.go",
					"renderer *inertia.Renderer",
				)
				assertCLITestFileContains(
					t,
					rootDir,
					"controllers/dashboards.go",
					"func NewDashboards(renderer *inertia.Renderer) Dashboards",
				)
				assertCLITestFileContains(
					t,
					rootDir,
					"controllers/dashboards.go",
					`return d.renderer.Page(etx, "Dashboard/Overview"`,
				)
			}
			assertCLITestFileExists(t, rootDir, tt.wantView)
			assertCLITestFileMissing(t, rootDir, tt.unwantedView)
		})
	}
}

func TestGenerateControllerCustomInertiaActionPreservesKeyedConstructor(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeCLITestFile(t, rootDir, "controllers/dashboards.go", `package controllers

type Dashboards struct {
	enabled bool
}

func NewDashboards(enabled bool) Dashboards {
	return Dashboards{enabled: enabled}
}
`)

	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
		Inertia:     "vue",
	}
	if err := lock.WriteLockFile(rootDir); err != nil {
		t.Fatalf("write andurel.lock: %v", err)
	}

	chdirCLITestRoot(t, rootDir)

	for range 2 {
		if err := generateControllerWithActions(
			"Dashboard",
			"",
			[]string{"overview"},
			"vue",
			false,
		); err != nil {
			t.Fatalf("generate custom Inertia controller action: %v", err)
		}
	}

	controllerPath := filepath.Join(rootDir, "controllers", "dashboards.go")
	content, err := os.ReadFile(controllerPath)
	if err != nil {
		t.Fatalf("read generated controller: %v", err)
	}
	controller := string(content)
	normalizedController := strings.Join(strings.Fields(controller), " ")
	for _, want := range []string{
		"type Dashboards struct { enabled bool renderer *inertia.Renderer }",
		"func NewDashboards(enabled bool, renderer *inertia.Renderer) Dashboards",
		"return Dashboards{enabled: enabled, renderer: renderer}",
	} {
		if !strings.Contains(normalizedController, want) {
			t.Errorf("generated controller missing %q:\n%s", want, controller)
		}
	}
	for _, want := range []string{
		`"github.com/mbvlabs/andurel/pkg/inertia"`,
		`return d.renderer.Page(etx, "Dashboard/Overview"`,
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("generated controller missing %q:\n%s", want, controller)
		}
	}
	for _, expected := range []struct {
		snippet string
		count   int
	}{
		{snippet: "renderer *inertia.Renderer", count: 2},
		{snippet: "func (d Dashboards) Overview", count: 1},
	} {
		if count := strings.Count(normalizedController, expected.snippet); count != expected.count {
			t.Errorf(
				"generated controller contains %q %d times, want %d:\n%s",
				expected.snippet,
				count,
				expected.count,
				controller,
			)
		}
	}
	if count := strings.Count(controller, `"github.com/mbvlabs/andurel/pkg/inertia"`); count != 1 {
		t.Errorf(
			"generated controller contains the Inertia import %d times, want once:\n%s",
			count,
			controller,
		)
	}
}

func TestGenerateControllerSingleCRUDActionVueGeneratesInertiaController(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	setupProjectInquiryCLITestProject(t, rootDir)
	chdirCLITestRoot(t, rootDir)

	if err := generateControllerWithActions(
		"ProjectInquiry",
		"",
		[]string{"show"},
		"vue",
		false,
	); err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"github.com/mbvlabs/andurel/pkg/inertia",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"renderer *inertia.Renderer",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		`return pi.renderer.Page(`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		`"ProjectInquiry/Show",`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"inertia.FromStruct(ProjectInquiryItemProps{",
	)
	assertCLITestFileNotContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"github.com/mbvlabs/andurel/pkg/hypermedia",
	)
	assertCLITestFileExists(
		t,
		rootDir,
		filepath.Join("resources", "js", "Pages", "ProjectInquiry", "Show.vue"),
	)
	assertCLITestFileMissing(t, rootDir, "views/project_inquiries_resource.templ")
}

func TestGenerateControllerSingleCRUDActionReactGeneratesInertiaController(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	setupProjectInquiryCLITestProject(t, rootDir)
	chdirCLITestRoot(t, rootDir)

	if err := generateControllerWithActions(
		"ProjectInquiry",
		"",
		[]string{"show"},
		"react",
		false,
	); err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"github.com/mbvlabs/andurel/pkg/inertia",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"renderer *inertia.Renderer",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		`return pi.renderer.Page(`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		`"ProjectInquiry/Show",`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"inertia.FromStruct(ProjectInquiryItemProps{",
	)
	assertCLITestFileNotContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"github.com/mbvlabs/andurel/pkg/hypermedia",
	)
	assertCLITestFileExists(
		t,
		rootDir,
		filepath.Join("resources", "js", "Pages", "ProjectInquiry", "Show.tsx"),
	)
	assertCLITestFileMissing(t, rootDir, "views/project_inquiries_resource.templ")
}

func setupProjectInquiryCLITestProject(t *testing.T, rootDir string) {
	t.Helper()

	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeCLITestFile(
		t,
		rootDir,
		"migrations/000100_create_project_inquiries.sql",
		`-- +goose Up
CREATE TABLE project_inquiries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE project_inquiries;
`,
	)
	writeCLITestFile(t, rootDir, "models/project_inquiry.go", "package models\n")
	writeCLITestFile(t, rootDir, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(rootDir, "bin", "templ"), 0o755); err != nil {
		t.Fatalf("chmod fake templ: %v", err)
	}

	lock := layout.NewAndurelLock("test")
	lock.DatabaseConfig = &layout.DatabaseConfig{
		Engine:   layout.DatabaseEnginePostgreSQL,
		NullType: layout.NullTypePGType,
	}
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName: "app",
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
	chdirCLITestRoot(t, rootDir)

	if err := generateControllerWithActions(
		"ProjectInquiry",
		"",
		[]string{"index"},
		"",
		false,
	); err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"github.com/mbvlabs/andurel/pkg/hypermedia",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"return hypermedia.RenderPage",
	)
	assertCLITestFileNotContains(
		t,
		rootDir,
		"controllers/project_inquiries.go",
		"github.com/mbvlabs/andurel/pkg/inertia",
	)
	assertCLITestFileExists(t, rootDir, "views/project_inquiries_resource.templ")
	assertCLITestFileContains(
		t,
		rootDir,
		"views/project_inquiries_resource.templ",
		"Items []models.ProjectInquiry",
	)
	assertCLITestFileNotContains(
		t,
		rootDir,
		"views/project_inquiries_resource.templ",
		"ProjectinquiryEntity",
	)
	assertCLITestFileMissing(
		t,
		rootDir,
		filepath.Join("resources", "js", "Pages", "ProjectInquiry", "Index.vue"),
	)
}

func TestGenerateControllerAPIWithNamespaceWritesUnderAPIPath(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	setupProjectInquiryCLITestProject(t, rootDir)
	writeEmptyControllersModule(t, rootDir)
	chdirCLITestRoot(t, rootDir)

	stdoutCapture := captureProcessOutput(t, &os.Stdout)
	err := generateControllerWithActions("v1/ProjectInquiry", "", []string{"create"}, "", true)
	stdout := stdoutCapture()
	if err != nil {
		t.Fatalf("generate controller: %v", err)
	}

	assertCLITestFileExists(
		t,
		rootDir,
		filepath.Join("controllers", "api", "v1", "project_inquiries.go"),
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "api", "v1", "project_inquiries.go"),
		"package v1",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("controllers", "api", "v1", "project_inquiries.go"),
		"routes.ApiV1ProjectInquiryCreate.Path()",
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("router", "routes", "api_v1_project_inquiries.go"),
		`const ApiV1ProjectInquiryPrefix = "/api/v1/project-inquiries"`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		filepath.Join("router", "routes", "api_v1_project_inquiries.go"),
		`"api.v1.project_inquiries.create"`,
	)
	assertCLITestFileContains(
		t,
		rootDir,
		"controllers/controller.go",
		`"example.com/app/controllers/api/v1"`,
	)
	assertCLITestFileContains(t, rootDir, "controllers/controller.go", "v1.NewProjectInquiries")
	assertCLITestFileMissing(t, rootDir, "views/api_v1_project_inquiries_resource.templ")
	if strings.Contains(stdout, "with views") {
		t.Fatalf("expected API generation output not to mention views, got:\n%s", stdout)
	}
}

func TestGenerateControllerRejectsModelNameForCustomOnly(t *testing.T) {
	resetCLITestSeams(t)
	rootDir := t.TempDir()
	writeCLITestFile(t, rootDir, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	chdirCLITestRoot(t, rootDir)

	err := generateControllerWithActions("Dashboard", "User", []string{"overview"}, "", false)
	if err == nil || !strings.Contains(err.Error(), "--model-name requires") {
		t.Fatalf("expected --model-name custom-only error, got %v", err)
	}
}

package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

// TestGeneratedHomePageIsPublic guards F01: the scaffold registers the welcome
// page without middleware.AuthOnly, and the runtime test suite keeps a
// protected-route regression next to it.
func TestGeneratedHomePageIsPublic(t *testing.T) {
	assertTemplateContains(t, "runtime_runtime.tmpl",
		"handlers.NewPagesHandler(renderer).Register(router)",
		"Pass middleware.AuthOnly(users) to Register to protect a page",
	)
	assertTemplateMissing(t, "runtime_runtime.tmpl",
		"NewPagesHandler(renderer).Register(router, middleware.AuthOnly(users))",
	)
	assertTemplateContains(t, "handlers_pages.tmpl",
		"is public: a new project must be reachable before anyone signs in",
	)
	assertTemplateContains(t, "runtime_runtime_test.tmpl",
		"func TestHomePageIsPublic(",
		"func TestAuthOnlyStillProtectsRoutes(",
	)
}

// TestGeneratedReadinessSeparatesLivenessAndReadiness guards F32's scaffold
// side: /api/ready is its own route, requires the SSR sidecar outside
// development, and never reuses the liveness wording.
func TestGeneratedReadinessSeparatesLivenessAndReadiness(t *testing.T) {
	assertTemplateContains(t, "router_routes_api.tmpl",
		"var Ready = routing.NewSimpleRoute(",
		`"/ready",`,
		`"api.ready",`,
	)
	assertTemplateContains(t, "handlers_api.tmpl",
		"func NewAPIHandler(",
		"rt.Get(routes.Health.Pattern(), a.health)",
		"rt.Get(routes.Ready.Pattern(), a.readiness)",
		// The probe writes its own response so a readiness failure stays 503
		// instead of becoming the themed 500 error page.
		"return anduin.JSON(ctx, http.StatusServiceUnavailable, map[string]string{",
	)
	assertTemplateContains(t, "runtime_runtime.tmpl",
		"handlers.NewAPIHandler(db, newReadiness(cfg, db)).Register(router)",
		"func newReadiness(cfg config.Config, db storage.Connection) readiness",
		"inertia.NewHTTPRenderer(",
		"return errors.New(\"inertia SSR renderer is unavailable\")",
		"return errors.New(\"database is unavailable\")",
	)
	assertTemplateContains(t, "runtime_runtime_test.tmpl",
		"func TestReadinessReportsReadyInDevelopment(",
		"func TestReadinessFailsWhenProductionSSRSidecarIsUnavailable(",
		"func TestReadinessRecoversWithHealthySSRSidecar(",
		"func TestReadinessIgnoresSSRWithExplicitCSRFallback(",
	)
	assertTemplateContains(t, "readme.tmpl", "`GET /api/ready`")
	assertTemplateContains(t, "agents.tmpl", "`GET /api/ready` is readiness")
}

// TestGeneratedConfigCheckIsReadOnly guards the F06 seam: doctor runs a
// generated helper instead of hard-coding required keys, and the helper only
// reads config and pings the database.
func TestGeneratedConfigCheckIsReadOnly(t *testing.T) {
	assertTemplateContains(t, "internal_configcheck_main.tmpl",
		"env.Load()",
		"config.New()",
		"storage.NewPostgres(ctx, cfg, storage.WithoutOpenTelemetry())",
		"db.Health(ctx)",
		`Status: "unreachable"`,
		"redactSecret(",
	)
	for _, forbidden := range []string{
		"RunMigrations",
		"CREATE DATABASE",
		"os.Remove",
		"os.WriteFile",
	} {
		assertTemplateMissing(t, "internal_configcheck_main.tmpl", forbidden)
	}
}

func TestGeneratedDocsShareOneQuickStart(t *testing.T) {
	for _, name := range []string{"readme.tmpl", "agents.tmpl"} {
		assertTemplateContains(t, name,
			"andurel tool sync",
			"cp .env.example .env",
			"andurel db migrate up",
			"andurel doctor",
		)
	}
	assertTemplateContains(t, "readme.tmpl",
		"{{.JavaScriptPackageManager}} install",
		"{{.JavaScriptFrozenInstallCommand}}",
		"{{.JavaScriptLockfileName}}",
	)
	assertTemplateMissing(t, "readme.tmpl", "createdb {{.Identifier}}")
	assertTemplateContains(t, "agents.tmpl", "Commit the package manager lockfile")
}

// TestReadmeRendersTheSelectedPackageManagerPolicy keeps the lockfile section
// truthful for every supported package manager.
func TestReadmeRendersTheSelectedPackageManagerPolicy(t *testing.T) {
	tests := []struct {
		manager  string
		lockfile string
		frozen   string
	}{
		{manager: "pnpm", lockfile: "pnpm-lock.yaml", frozen: "pnpm install --frozen-lockfile"},
		{manager: "npm", lockfile: "package-lock.json", frozen: "npm ci"},
		{manager: "bun", lockfile: "bun.lock", frozen: "bun install --frozen-lockfile"},
	}

	for _, test := range tests {
		t.Run(test.manager, func(t *testing.T) {
			root := t.TempDir()
			data := &TemplateData{
				AppName:                  "testapp",
				ProjectName:              "testapp",
				ModuleName:               "testapp",
				GoVersion:                goVersion,
				Inertia:                  "react",
				JavaScriptPackageManager: test.manager,
				blueprint:                initializeBlueprint("testapp"),
			}
			if err := renderTemplate(
				root, "readme.tmpl", "README.md", layouttemplates.Files, data,
			); err != nil {
				t.Fatalf("render README: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(root, "README.md"))
			if err != nil {
				t.Fatalf("read README: %v", err)
			}
			for _, want := range []string{
				test.lockfile,
				test.frozen,
				test.manager + " install",
				"andurel tool sync",
				"`GET /api/ready`",
			} {
				if !strings.Contains(string(body), want) {
					t.Fatalf("README for %s missing %q", test.manager, want)
				}
			}
		})
	}
}

// TestConfigCheckTemplateRendersAsGo keeps the generated helper buildable: the
// scaffold gofmt test only covers templates in the base mappings, and this test
// asserts the mapping and a parseable render.
func TestConfigCheckTemplateRendersAsGo(t *testing.T) {
	mapping, ok := baseTemplateMappings["internal_configcheck_main.tmpl"]
	if !ok || string(mapping) != "internal/configcheck/main.go" {
		t.Fatalf("config check template mapping = %q, %t", mapping, ok)
	}

	body, err := layouttemplates.Files.ReadFile("internal_configcheck_main.tmpl")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	if strings.Contains(string(body), "{{.") && !strings.Contains(string(body), "{{.ModuleName}}") {
		t.Fatalf("unexpected template action in config check:\n%s", body)
	}

	root := t.TempDir()
	if err := renderTemplate(
		root,
		"internal_configcheck_main.tmpl",
		"internal/configcheck/main.go",
		layouttemplates.Files,
		&TemplateData{ModuleName: "testapp"},
	); err != nil {
		t.Fatalf("render config check: %v", err)
	}
	rendered, err := os.ReadFile(filepath.Join(root, "internal/configcheck/main.go"))
	if err != nil {
		t.Fatalf("read rendered config check: %v", err)
	}
	if !strings.Contains(string(rendered), `"testapp/config"`) {
		t.Fatalf("rendered config check does not import the project config:\n%s", rendered)
	}
}

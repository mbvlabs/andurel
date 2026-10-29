package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/cli/patterns"
	"github.com/mbvlabs/andurel/v2/layout"
)

func TestRenderRoutesJSGeneratesDeterministicHelpers(t *testing.T) {
	manifest := routeManifest{
		Routes: []routeManifestRoute{
			{
				Variable:  "WidgetShow",
				Path:      "/widgets/:id",
				IsInertia: true,
				Params:    []routeManifestParam{{Name: "id", Type: "uuid"}},
			},
			{
				Variable:  "SessionCreate",
				Path:      "/users/sign-in",
				IsInertia: true,
			},
			{
				Variable:  "DashboardLookup",
				Path:      "/admin/teams/:team_id/dashboards/:dashboard_id",
				IsInertia: true,
				Params: []routeManifestParam{
					{Name: "team_id", Type: "int32"},
					{Name: "dashboard_id", Type: "int64"},
				},
			},
			{
				Variable:  "PasswordEdit",
				Path:      "/users/password/:token/edit",
				IsInertia: true,
				Params:    []routeManifestParam{{Name: "token", Type: "string"}},
			},
			{
				Variable:  "ArticleShow",
				Path:      "/articles/:slug",
				IsInertia: true,
				Params:    []routeManifestParam{{Name: "slug", Type: "unknown"}},
			},
		},
	}

	got, count, err := renderRoutesJS(manifest)
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 helpers, got %d", count)
	}

	want := routesJSPreamble +
		"  articleShow: routeHelper({\n    host: 'primary',\n    path: (slug: string) => `/articles/${slug}`,\n  }),\n" +
		"  dashboardLookup: routeHelper({\n    host: 'primary',\n    path: (team_id: number, dashboard_id: number) => `/admin/teams/${team_id}/dashboards/${dashboard_id}`,\n  }),\n" +
		"  passwordEdit: routeHelper({\n    host: 'primary',\n    path: (token: string) => `/users/password/${token}/edit`,\n  }),\n" +
		"  sessionCreate: routeHelper({\n    host: 'primary',\n    path: () => '/users/sign-in',\n  }),\n" +
		"  widgetShow: routeHelper({\n    host: 'primary',\n    path: (id: string) => `/widgets/${id}`,\n  }),\n" +
		"}\n" +
		routesJSHelperRuntime
	if string(got) != want {
		t.Fatalf("unexpected routes.ts:\n%s", got)
	}
}

// TestRoutePatternMatchesGeneratedRoutesTS keeps the route pattern's
// resources/js/routes.ts sample byte-identical to what `andurel sync routes`
// writes for the same route data.
func TestRoutePatternMatchesGeneratedRoutesTS(t *testing.T) {
	pattern, ok := patterns.Get("route")
	if !ok {
		t.Fatal("route pattern not found")
	}

	var want string
	for _, file := range pattern.Files {
		if file.Path == generatedRoutesJSPath {
			want = file.Content
			break
		}
	}
	if want == "" {
		t.Fatalf("route pattern has no %s sample", generatedRoutesJSPath)
	}

	got, _, err := renderRoutesJS(routeManifest{
		Routes: []routeManifestRoute{
			{
				Variable:  "UserIndex",
				Path:      "/users",
				IsInertia: true,
			},
			{
				Variable:  "UserShow",
				Path:      "/users/:id",
				IsInertia: true,
				Params:    []routeManifestParam{{Name: "id", Type: "string"}},
			},
			{
				Variable:  "UserCreate",
				Path:      "/users",
				IsInertia: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}
	if string(got) != want {
		t.Fatalf("route pattern sample is stale:\n--- generated ---\n%s\n--- pattern ---\n%s", got, want)
	}
}

func TestRenderRoutesJSIncludesSecondaryHost(t *testing.T) {
	manifest := routeManifest{
		Routes: []routeManifestRoute{
			{
				Variable:  "HomePage",
				Path:      "/",
				IsInertia: true,
			},
			{
				Variable:  "WidgetIndex",
				Path:      "/widgets",
				Host:      "admin",
				IsInertia: true,
			},
		},
	}

	got, count, err := renderRoutesJS(manifest)
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 helpers, got %d", count)
	}
	for _, want := range []string{
		"homePage: routeHelper({\n    host: 'primary',\n    path: () => '/',\n  })",
		"widgetIndex: routeHelper({\n    host: 'admin',\n    path: () => '/widgets',\n  })",
		"export function configureRouteHosts",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestRenderRoutesJSUsesURLCShapedPathForExactRoutes(t *testing.T) {
	manifest := routeManifest{
		Routes: []routeManifestRoute{{
			Variable:  "HomePage",
			Path:      "/",
			Pattern:   "/{$}",
			Exact:     true,
			IsInertia: true,
		}},
	}

	got, _, err := renderRoutesJS(manifest)
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}
	if strings.Contains(string(got), "{$}") {
		t.Fatalf("helpers must emit the URL path, not the mux pattern:\n%s", got)
	}
	if want := "homePage: routeHelper({\n    host: 'primary',\n    path: () => '/',\n  })"; !strings.Contains(
		string(got),
		want,
	) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, got)
	}

	// Defensive: a mux pattern must never leak into the URL helper.
	if got := routeJSURLPath(routeManifestRoute{Path: "/{$}"}); got != "/" {
		t.Fatalf("routeJSURLPath(%q) = %q, want %q", "/{$}", got, "/")
	}
}

func TestRenderRoutesJSEscapesStaticPathSegments(t *testing.T) {
	manifest := routeManifest{
		Routes: []routeManifestRoute{
			{
				Variable:  "AssetShow",
				Path:      "/assets/it's-$${fine}/:name/`raw`",
				IsInertia: true,
				Params:    []routeManifestParam{{Name: "name", Type: "string"}},
			},
			{
				Variable:  "QuoteShow",
				Path:      "/quotes/it's-ok",
				IsInertia: true,
			},
		},
	}

	got, _, err := renderRoutesJS(manifest)
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}

	for _, want := range []string{
		"path: (name: string) => `/assets/it's-$\\${fine}/${name}/\\`raw\\``",
		"path: () => '/quotes/it\\'s-ok'",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestRenderRoutesJSSkipsRoutesWithoutIsInertia(t *testing.T) {
	manifest := routeManifest{
		Routes: []routeManifestRoute{
			{Variable: "Health", Name: "api.health", Path: "/api/health"},
			{Variable: "Robots", Name: "assets.robots", Path: "/robots.txt"},
			{Variable: "Sitemap", Name: "assets.sitemap", Path: "/sitemap.xml"},
			{Variable: "ViteBuild", Name: "vite.build", Path: "/assets/dist/*"},
			{
				Variable:  "SessionNew",
				Name:      "users.new_user_session",
				Path:      "/users/sign-in",
				IsInertia: true,
			},
		},
	}

	got, count, err := renderRoutesJS(manifest)
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 helper, got %d:\n%s", count, got)
	}
	if !strings.Contains(string(got), "path: () => '/users/sign-in'") {
		t.Fatalf("expected session route helper, got:\n%s", got)
	}
	for _, unwanted := range []string{"health", "robots", "sitemap", "viteBuild"} {
		if strings.Contains(string(got), unwanted) {
			t.Fatalf("expected generated routes.ts to omit %s, got:\n%s", unwanted, got)
		}
	}
}

func TestRenderRoutesJSDetectsHelperNameCollision(t *testing.T) {
	manifest := routeManifest{
		Routes: []routeManifestRoute{
			{Variable: "SessionCreate", Path: "/session", IsInertia: true},
			{Variable: "sessionCreate", Path: "/session/new", IsInertia: true},
		},
	}

	_, _, err := renderRoutesJS(manifest)
	if err == nil {
		t.Fatal("expected helper name collision error")
	}
	if !strings.Contains(err.Error(), "route helper name collision") {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestGenerateRoutesJSFileCreatesDirectoryAndOverwritesFile(t *testing.T) {
	rootDir := t.TempDir()
	target := filepath.Join(rootDir, generatedRoutesJSPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create resources/js: %v", err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("write existing routes.ts: %v", err)
	}
	manifest := routeManifest{
		Routes: []routeManifestRoute{{
			Variable:  "SessionCreate",
			Path:      "/users/sign-in",
			IsInertia: true,
		}},
		Skipped: []routeManifestSkipped{{
			Variable: "DynamicRoute",
			Reason:   "route path is not a static string expression",
		}},
	}

	report, err := generateRoutesJSFile(rootDir, manifest)
	if err != nil {
		t.Fatalf("generate routes: %v", err)
	}
	if report.GeneratedFile != generatedRoutesJSPath || report.GeneratedHelpers != 1 ||
		report.SkippedCount != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read generated routes.ts: %v", err)
	}
	if !strings.Contains(string(got), "path: () => '/users/sign-in'") {
		t.Fatalf("unexpected generated routes.ts:\n%s", got)
	}
	if !strings.Contains(string(got), "configureRouteHosts") {
		t.Fatalf("expected configureRouteHosts in generated routes.ts:\n%s", got)
	}
}

func TestGenerateRoutesCommandRequiresInertiaProject(t *testing.T) {
	rootDir := setupRoutesJSCommandProject(t, "")

	result := runRoutesJSCommandInProject(t, rootDir, "sync", "routes", "--json")
	if result.err == nil {
		t.Fatal("expected generate routes to reject non-Inertia project")
	}
	envelope := output.Fail(result.err)
	if envelope.Code != output.CodeInvalidInertiaAdapter {
		t.Fatalf("expected invalid inertia error, got %#v", envelope)
	}
	if _, err := os.Stat(filepath.Join(rootDir, generatedRoutesJSPath)); !os.IsNotExist(err) {
		t.Fatalf("expected routes.ts not to be generated, stat err: %v", err)
	}
}

func TestConfiguredInertiaAdapterRejectsMissingAndInvalidConfiguration(t *testing.T) {
	root := t.TempDir()
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if _, err := configuredInertiaAdapter(root); err == nil {
		t.Fatal("expected missing Inertia adapter to fail")
	} else if envelope := output.Fail(err); envelope.Code != output.CodeInvalidInertiaAdapter {
		t.Fatalf("missing adapter error = %#v", envelope)
	}

	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:              "app",
		Inertia:                  "svelte",
		JavaScriptPackageManager: "pnpm",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write valid lock: %v", err)
	}
	adapter, err := configuredInertiaAdapter(root)
	if err != nil || adapter != "svelte" {
		t.Fatalf("configured adapter = %q, %v", adapter, err)
	}
}

func TestGenerateRoutesCommandAllowsInertiaProject(t *testing.T) {
	rootDir := setupRoutesJSCommandProject(t, "vue")

	result := runRoutesJSCommandInProject(t, rootDir, "sync", "routes", "--json")
	if result.err != nil {
		t.Fatalf("generate routes failed: %v\nstderr:\n%s", result.err, result.stderr)
	}

	target := filepath.Join(rootDir, generatedRoutesJSPath)
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read generated routes.ts: %v", err)
	}
	if !strings.Contains(string(got), "path: () => '/users/sign-in'") {
		t.Fatalf("unexpected generated routes.ts:\n%s", got)
	}
	for _, want := range []string{
		"routing.TypeScriptRoute()",
		"configureRouteHosts",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected generated routes.ts to contain %q:\n%s", want, got)
		}
	}
	if !strings.Contains(result.stdout, `"generated_file": "resources/js/routes.ts"`) {
		t.Fatalf("expected structured generation report, got:\n%s", result.stdout)
	}
}

func TestCodeGenerationChecksSkipsRoutesTSForNonInertiaProject(t *testing.T) {
	rootDir := setupRoutesJSCommandProject(t, "")

	results := codeGenerationChecks(rootDir, false)
	for _, result := range results {
		if result.name == "routes.ts" {
			t.Fatalf("expected routes.ts check to be skipped for non-Inertia project: %#v", results)
		}
	}
}

func TestCheckRoutesTSGeneratePassesWhenCurrent(t *testing.T) {
	rootDir := setupRoutesJSCommandProject(t, "react")
	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		t.Fatalf("collect route manifest: %v", err)
	}
	if _, err := generateRoutesJSFile(rootDir, manifest); err != nil {
		t.Fatalf("generate routes.ts: %v", err)
	}

	result := checkRoutesTSGenerate(rootDir, false)
	if result.status != statusPass {
		t.Fatalf("expected routes.ts check to pass, got %#v", result)
	}
}

func TestCheckRoutesTSGenerateFailsWhenMissingOrStale(t *testing.T) {
	rootDir := setupRoutesJSCommandProject(t, "vue")

	missing := checkRoutesTSGenerate(rootDir, false)
	if missing.status != statusFail || !strings.Contains(missing.message, "missing") {
		t.Fatalf("expected missing routes.ts failure, got %#v", missing)
	}

	target := filepath.Join(rootDir, generatedRoutesJSPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create resources/js: %v", err)
	}
	if err := os.WriteFile(target, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale routes.ts: %v", err)
	}

	stale := checkRoutesTSGenerate(rootDir, false)
	if stale.status != statusFail || !strings.Contains(stale.message, "out of date") {
		t.Fatalf("expected stale routes.ts failure, got %#v", stale)
	}
}

func setupRoutesJSCommandProject(t *testing.T, inertia string) string {
	t.Helper()

	rootDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootDir, "go.mod"),
		[]byte("module example.com/app\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	lock := layout.AndurelLock{
		SchemaVersion: 1,
		Version:       "test",
		Tools:         map[string]*layout.Tool{},
		ScaffoldConfig: &layout.ScaffoldConfig{
			ProjectName: "app",
			Inertia:     inertia,
		},
		DatabaseConfig: &layout.DatabaseConfig{
			Engine:   layout.DatabaseEnginePostgreSQL,
			NullType: layout.NullTypePGType,
		},
	}
	if err := lock.WriteLockFile(rootDir); err != nil {
		t.Fatalf("write project files: %v", err)
	}
	writeRouteManifestTestFile(t, rootDir, "users.go", `package routes

import "example.com/app/pkg/routing"

const UserPrefix = "/users"

var SessionCreate = routing.NewSimpleRoute(
	"/sign-in",
	"users.user_session",
	UserPrefix,
	routing.InertiaRoute(),
)
`)
	return rootDir
}

func runRoutesJSCommandInProject(t *testing.T, rootDir string, args ...string) cliTestResult {
	t.Helper()
	resetCLITestSeams(t)
	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return cliTestResult{
		cmd:    cmd,
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func TestRenderRoutesJSRequiresConfiguredHosts(t *testing.T) {
	got, _, err := renderRoutesJS(routeManifest{
		Routes: []routeManifestRoute{{Variable: "HomePage", Path: "/", IsInertia: true}},
	})
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}

	for _, want := range []string{
		"export type RouteHostScope",
		"export function configureRouteHostScope",
		"export function withRouteHosts",
		"function currentOrigins(): HostOrigins",
		"route hosts are not configured yet",
		"use routes.<name>.path() for a relative URL",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected generated routes.ts to contain %q, got:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"let origins: HostOrigins = {}", "?? ''"} {
		if strings.Contains(string(got), unwanted) {
			t.Fatalf("generated routes.ts still degrades silently with %q:\n%s", unwanted, got)
		}
	}
}

// routesHostIsolationProbe exercises the generated module in a real Node
// process. It asserts the runtime behavior static assertions cannot prove:
// concurrent asynchronous renders keep separate host origins and a failed
// render cannot leak its hosts into the next one.
const routesHostIsolationProbe = `import { AsyncLocalStorage } from 'node:async_hooks'
import {
  configureRouteHostScope,
  configureRouteHosts,
  withRouteHosts,
  routes,
} from './routes.ts'

const storage = new AsyncLocalStorage()
configureRouteHostScope({
  current: () => storage.getStore(),
  run: (hosts, render) => storage.run(hosts, render),
})

function check(name, condition, detail) {
  if (condition) {
    console.log('ok ' + name)
    return
  }
  console.error('FAIL ' + name + (detail ? ': ' + detail : ''))
  process.exitCode = 1
}

function thrown(render) {
  try {
    render()
  } catch (error) {
    return error
  }
  return null
}

const unconfigured = thrown(() => routes.homePage())
check(
  'unconfigured-full-url-throws',
  unconfigured !== null &&
    unconfigured.message.includes('configureRouteHosts') &&
    unconfigured.message.includes('.path()'),
  unconfigured === null ? 'no error' : unconfigured.message,
)

check(
  'path-works-before-configuration',
  routes.homePage.path() === '/' && routes.adminWidget.path() === '/widgets',
)

const missingOrigin = thrown(() => withRouteHosts({}, () => routes.homePage()))
check(
  'missing-primary-origin-throws',
  missingOrigin !== null && missingOrigin.message.includes('"primary"'),
  missingOrigin === null ? 'no error' : missingOrigin.message,
)

const gate = () => new Promise(resolve => setImmediate(resolve))

function render(hosts) {
  return withRouteHosts(hosts, async () => {
    await gate()
    return routes.homePage()
  })
}

const [a, b, c] = await Promise.all([
  render({ primary: 'https://a.test' }),
  render({ primary: 'https://b.test', admin: 'https://admin.b.test' }),
  render({ primary: 'https://c.test' }),
])
check(
  'concurrent-renders-stay-isolated',
  a === 'https://a.test/' && b === 'https://b.test/' && c === 'https://c.test/',
  [a, b, c].join(' '),
)

const failing = withRouteHosts({ primary: 'https://broken.test' }, async () => {
  await gate()
  throw new Error('render failed')
})
const afterFailure = render({ primary: 'https://after.test' })
const failure = await failing.then(() => null, error => error)
check(
  'failed-render-does-not-leak-hosts',
  failure instanceof Error && (await afterFailure) === 'https://after.test/',
)

configureRouteHosts({ primary: 'https://browser.test', admin: 'https://admin.test' })
check(
  'browser-configuration-resolves',
  routes.homePage() === 'https://browser.test/' &&
    routes.adminWidget() === 'https://admin.test/widgets',
)

configureRouteHostScope(null)
const missingScope = thrown(() => withRouteHosts({ primary: 'https://x.test' }, () => '/'))
check(
  'without-scope-throws',
  missingScope !== null && missingScope.message.includes('configureRouteHostScope'),
  missingScope === null ? 'no error' : missingScope.message,
)
`

func TestGeneratedRoutesTSIsolatesConcurrentSSRHosts(t *testing.T) {
	node := requireNodeForRoutesProbe(t)

	content, _, err := renderRoutesJS(routeManifest{
		Routes: []routeManifestRoute{
			{Variable: "HomePage", Path: "/", IsInertia: true},
			{Variable: "AdminWidget", Path: "/widgets", Host: "admin", IsInertia: true},
		},
	})
	if err != nil {
		t.Fatalf("render routes js: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "routes.ts"),
		content,
		0o644,
	); err != nil {
		t.Fatalf("write routes.ts: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "probe.mjs"),
		[]byte(routesHostIsolationProbe),
		0o644,
	); err != nil {
		t.Fatalf("write probe: %v", err)
	}

	command := exec.Command(node, "--experimental-strip-types", "probe.mjs")
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil && strings.Contains(string(output), "bad option") {
		t.Skipf("skipping generated routes.ts runtime probe: %s does not accept --experimental-strip-types:\n%s", node, output)
	}
	if err != nil {
		t.Fatalf("generated routes.ts probe failed: %v\n%s", err, output)
	}

	for _, want := range []string{
		"ok unconfigured-full-url-throws",
		"ok path-works-before-configuration",
		"ok missing-primary-origin-throws",
		"ok concurrent-renders-stay-isolated",
		"ok failed-render-does-not-leak-hosts",
		"ok browser-configuration-resolves",
		"ok without-scope-throws",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("probe output missing %q:\n%s", want, output)
		}
	}
}

// requireNodeForRoutesProbe returns a Node.js that can execute the generated
// TypeScript module. The probe is skipped with its reason when the toolchain is
// unavailable; the generated-module assertions still run everywhere.
func requireNodeForRoutesProbe(t *testing.T) string {
	t.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("skipping generated routes.ts runtime probe: node is not installed")
	}

	version, err := exec.Command(node, "--version").Output()
	if err != nil {
		t.Skipf("skipping generated routes.ts runtime probe: node --version failed: %v", err)
	}

	var major, minor int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(version)), "v%d.%d", &major, &minor); err != nil {
		t.Skipf("skipping generated routes.ts runtime probe: unrecognized node version %q", version)
	}
	if major < 22 || (major == 22 && minor < 6) {
		t.Skipf(
			"skipping generated routes.ts runtime probe: node %s predates --experimental-strip-types",
			strings.TrimSpace(string(version)),
		)
	}

	return node
}

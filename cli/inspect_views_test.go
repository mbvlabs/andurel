package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

const inspectViewsGoMod = `module example.com/app

go 1.27.1
`

const inspectViewsRoutes = `package routes

import (
	"example.com/app/config"

	"github.com/mbvlabs/andurel/pkg/routing"
)

const UserPrefix = "/users"

var HomePage = routing.NewSimpleRoute(
	"/",
	"pages.home",
	"",
	routing.TypeScriptRoute(),
	routing.Exact(),
)

var Health = routing.NewSimpleRoute(
	"/health",
	"api.health",
	"/api",
)

var PasswordEdit = routing.NewRouteWithToken(
	"/password/:token/edit",
	"users.edit_user_password",
	UserPrefix,
	routing.TypeScriptRoute(),
)

var AdminHome = routing.NewSimpleRoute(
	"/",
	"admin.home",
	"",
	routing.TypeScriptRoute(),
	routing.Host(config.HostAdmin),
)
`

const inspectViewsHosts = `package config

import "github.com/mbvlabs/andurel/pkg/routing"

const HostAdmin routing.HostName = "admin"
`

const inspectViewsHandlers = `package handlers

import (
	"context"

	"example.com/app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

type PagesHandler struct {
	renderer *inertia.Renderer
}

func NewPagesHandler(renderer *inertia.Renderer) PagesHandler {
	return PagesHandler{renderer: renderer}
}

func (p PagesHandler) Register(rt *anduin.Router) {
	rt.Get(routes.HomePage.Pattern(), p.Home)
	rt.Get(routes.PasswordEdit.Pattern(), p.PasswordEdit)
	rt.Get(routes.AdminHome.Pattern(), p.Admin)
	rt.Get(routes.Health.Pattern(), p.Health)
}

func (p PagesHandler) Home(ctx context.Context) error {
	return p.renderer.Page(ctx, "Welcome", inertia.Props{}).Render()
}

func (p PagesHandler) PasswordEdit(ctx context.Context) error {
	return p.renderer.Page(ctx, "Auth/ResetPassword", inertia.Props{}).SSR().Render()
}

func (p PagesHandler) Admin(ctx context.Context) error {
	return p.renderer.Page(ctx, componentForAdmin(), inertia.Props{}).Render()
}

func (p PagesHandler) Health(context.Context) error {
	return nil
}

func componentForAdmin() string {
	return "Admin/Home"
}
`

const inspectViewsAppConfig = `package config

import "github.com/mbvlabs/andurel/pkg/env"

func NewApp() {
	e := env.New()
	_ = e.String("PROTOCOL", "")
	_ = e.RequiredString("HOST_PRIMARY")
}
`

const inspectViewsInertiaConfig = `package config

import "github.com/mbvlabs/andurel/pkg/env"

func NewInertia() {
	e := env.New()
	_ = e.String("INERTIA_SSR_LISTEN", "http://127.0.0.1:13714")
	_ = e.String("INERTIA_SSR_URL", "http://127.0.0.1:13714")
}
`

func writeInspectViewsProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", inspectViewsGoMod)
	writeTestFile(t, root, "routes/pages.go", inspectViewsRoutes)
	writeTestFile(t, root, "config/hosts.go", inspectViewsHosts)
	writeTestFile(t, root, "config/app.go", inspectViewsAppConfig)
	writeTestFile(t, root, "config/inertia.go", inspectViewsInertiaConfig)
	writeTestFile(t, root, "handlers/pages.go", inspectViewsHandlers)
	writeTestFile(t, root, "resources/js/Pages/Welcome.tsx", "export default function Welcome() { return null }\n")
	writeTestFile(t, root, "resources/js/Pages/Auth/Login.tsx", "export default function Login() { return null }\n")
	writeTestFile(t, root, "resources/js/Pages/Auth/ResetPassword.tsx", "export default function ResetPassword() { return null }\n")
	writeTestFile(t, root, "resources/js/Pages/Errors/NotFound.tsx", "export default function NotFound() { return null }\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n")
	writeTestFile(t, root, "cmd/queue/main.go", "package main\n")
	writeTestFile(t, root, "cmd/migrate/main.go", "package main\n")
	writeTestFile(t, root, "cmd/ssr/main.go", "package main\n")
	writeTestFile(t, root, "cmd/seeds/main.go", "package main\n")

	lock := layout.NewAndurelLock("v1.2.3")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:              "app",
		Inertia:                  "react",
		JavaScriptPackageManager: "pnpm",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	return root
}

func runInspectJSON(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	result := runCLIOnProject(t, root, append(args, "--json")...)
	if result.err != nil {
		t.Fatalf("%v: %v\nstdout:\n%s\nstderr:\n%s", args, result.err, result.stdout, result.stderr)
	}
	return []byte(result.stdout)
}

func runCLIOnProject(t *testing.T, root string, args ...string) cliTestResult {
	t.Helper()
	resetCLITestSeams(t)
	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = originalFindGoModRoot })

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return cliTestResult{cmd: cmd, stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func decodeInspectEnvelope[T any](t *testing.T, output []byte) T {
	t.Helper()
	var envelope struct {
		OK   bool `json:"ok"`
		Data T    `json:"data"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, output)
	}
	if !envelope.OK {
		t.Fatalf("inspect output not ok:\n%s", output)
	}
	return envelope.Data
}

type inspectURLRoute struct {
	Name             string   `json:"name"`
	Methods          []string `json:"methods"`
	Path             string   `json:"path"`
	Pattern          string   `json:"pattern"`
	Host             string   `json:"host"`
	HostConfigKey    string   `json:"host_config_key"`
	Origin           string   `json:"origin"`
	OriginSource     string   `json:"origin_source"`
	URL              string   `json:"url"`
	URLTemplate      string   `json:"url_template"`
	Dynamic          bool     `json:"dynamic"`
	Resolved         bool     `json:"resolved"`
	UnresolvedReason string   `json:"unresolved_reason"`
}

type inspectURLsReport struct {
	Routes []inspectURLRoute `json:"routes"`
}

func TestInspectURLsResolvesKnownHostsAndReportsMissingOnes(t *testing.T) {
	root := writeInspectViewsProject(t)
	writeTestFile(t, root, ".env", "HOST_PRIMARY=localhost:8080\nPROTOCOL=http\n")
	unsetEnvForTest(t, "HOST_PRIMARY", "HOST_ADMIN", "PROTOCOL")

	report := decodeInspectEnvelope[inspectURLsReport](
		t,
		runInspectJSON(t, root, "inspect", "urls"),
	)
	byName := map[string]inspectURLRoute{}
	for _, route := range report.Routes {
		byName[route.Name] = route
	}

	home, ok := byName["pages.home"]
	if !ok {
		t.Fatalf("missing pages.home in %#v", report.Routes)
	}
	if home.Host != "primary" || !home.Resolved {
		t.Fatalf("pages.home = %#v", home)
	}
	if home.Origin != "http://localhost:8080" {
		t.Fatalf("pages.home origin = %q", home.Origin)
	}
	if !strings.Contains(home.OriginSource, "HOST_PRIMARY") ||
		!strings.Contains(home.OriginSource, "PROTOCOL") {
		t.Fatalf("pages.home origin source = %q", home.OriginSource)
	}
	if home.URL != "http://localhost:8080/" {
		t.Fatalf("pages.home url = %q", home.URL)
	}
	if len(home.Methods) != 1 || home.Methods[0] != "GET" {
		t.Fatalf("pages.home methods = %#v", home.Methods)
	}

	password, ok := byName["users.edit_user_password"]
	if !ok {
		t.Fatalf("missing users.edit_user_password in %#v", report.Routes)
	}
	if !password.Dynamic || password.URL != "" ||
		password.URLTemplate != "http://localhost:8080/users/password/:token/edit" {
		t.Fatalf("password route = %#v", password)
	}

	admin, ok := byName["admin.home"]
	if !ok {
		t.Fatalf("missing admin.home in %#v", report.Routes)
	}
	if admin.Host != "admin" || admin.Resolved || admin.URL != "" {
		t.Fatalf("admin.home = %#v", admin)
	}
	if !strings.Contains(admin.UnresolvedReason, "HOST_ADMIN") {
		t.Fatalf("admin.home reason = %q", admin.UnresolvedReason)
	}
}

func TestInspectURLsDoesNotInventMissingConfiguration(t *testing.T) {
	root := writeInspectViewsProject(t)
	unsetEnvForTest(t, "HOST_PRIMARY", "HOST_ADMIN", "PROTOCOL")

	report := decodeInspectEnvelope[inspectURLsReport](
		t,
		runInspectJSON(t, root, "inspect", "urls"),
	)
	for _, route := range report.Routes {
		if route.Resolved {
			t.Fatalf("route %s resolved without host configuration: %#v", route.Name, route)
		}
		if route.URL != "" || strings.Contains(route.URL, "localhost") {
			t.Fatalf("route %s invented a URL: %#v", route.Name, route)
		}
		if !strings.Contains(route.UnresolvedReason, route.HostConfigKey) {
			t.Fatalf("route %s reason = %q, want config key %s", route.Name, route.UnresolvedReason, route.HostConfigKey)
		}
	}
}

type inspectPageRoute struct {
	Name          string `json:"name"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Component     string `json:"component"`
	ComponentFile string `json:"component_file"`
	ComponentOK   bool   `json:"component_resolved"`
	SSRPolicy     string `json:"ssr_policy"`
	SSRReason     string `json:"ssr_reason"`
}

type inspectPageComponent struct {
	Component string             `json:"component"`
	File      string             `json:"file"`
	Exists    bool               `json:"exists"`
	Routes    []inspectPageRoute `json:"routes"`
}

type inspectUnresolvedPage struct {
	Handler    string `json:"handler"`
	Method     string `json:"method"`
	SourceFile string `json:"source_file"`
	Line       int    `json:"line"`
	Reason     string `json:"reason"`
}

type inspectPagesReport struct {
	Pages      []inspectPageComponent  `json:"pages"`
	Unresolved []inspectUnresolvedPage `json:"unresolved"`
}

func TestInspectPagesListsComponentsRoutesAndSSRPolicy(t *testing.T) {
	root := writeInspectViewsProject(t)

	report := decodeInspectEnvelope[inspectPagesReport](
		t,
		runInspectJSON(t, root, "inspect", "pages"),
	)
	byComponent := map[string]inspectPageComponent{}
	for _, page := range report.Pages {
		byComponent[page.Component] = page
	}
	if len(report.Pages) != 4 {
		t.Fatalf("pages = %#v", report.Pages)
	}
	for _, component := range []string{"Welcome", "Auth/Login", "Auth/ResetPassword", "Errors/NotFound"} {
		page, ok := byComponent[component]
		if !ok || !page.Exists || page.File == "" {
			t.Fatalf("component %q = %#v", component, page)
		}
	}

	welcome := byComponent["Welcome"]
	if len(welcome.Routes) != 1 {
		t.Fatalf("Welcome routes = %#v", welcome.Routes)
	}
	if welcome.Routes[0].Name != "pages.home" || welcome.Routes[0].Method != "GET" ||
		welcome.Routes[0].Path != "/" || !welcome.Routes[0].ComponentOK {
		t.Fatalf("Welcome route = %#v", welcome.Routes[0])
	}
	if welcome.Routes[0].SSRPolicy != "unresolved" ||
		!strings.Contains(welcome.Routes[0].SSRReason, "runtime") {
		t.Fatalf("Welcome SSR policy = %#v", welcome.Routes[0])
	}

	reset := byComponent["Auth/ResetPassword"]
	if len(reset.Routes) != 1 || reset.Routes[0].SSRPolicy != "enabled" {
		t.Fatalf("Auth/ResetPassword = %#v", reset)
	}

	if len(byComponent["Auth/Login"].Routes) != 0 {
		t.Fatalf("Auth/Login should have no static route association: %#v", byComponent["Auth/Login"])
	}

	if len(report.Unresolved) != 1 {
		t.Fatalf("unresolved = %#v", report.Unresolved)
	}
	unresolved := report.Unresolved[0]
	if unresolved.Handler != "PagesHandler" || unresolved.Method != "Admin" ||
		unresolved.SourceFile != "handlers/pages.go" || unresolved.Line == 0 ||
		!strings.Contains(unresolved.Reason, "not a static string") {
		t.Fatalf("unresolved = %#v", unresolved)
	}
}

type inspectProcessReport struct {
	Processes []inspectProcess `json:"processes"`
}

type inspectProcess struct {
	Name                string   `json:"name"`
	Source              string   `json:"source"`
	Buildable           bool     `json:"buildable"`
	Responsibility      string   `json:"responsibility"`
	ResponsibilityKnown bool     `json:"responsibility_known"`
	ConfigKeys          []string `json:"config_keys"`
	Prerequisites       []string `json:"prerequisites"`
	StartupOrder        int      `json:"startup_order"`
	StartupNote         string   `json:"startup_note"`
	HealthEndpoint      string   `json:"health_endpoint"`
	HealthResolved      bool     `json:"health_resolved"`
	HealthReason        string   `json:"health_reason"`
}

func TestInspectProcessReportsResponsibilitiesOrderAndHealth(t *testing.T) {
	root := writeInspectViewsProject(t)
	writeTestFile(t, root, ".env", "HOST_PRIMARY=localhost:8080\nPROTOCOL=http\n")
	unsetEnvForTest(t, "INERTIA_SSR_LISTEN", "INERTIA_SSR_URL")

	report := decodeInspectEnvelope[inspectProcessReport](
		t,
		runInspectJSON(t, root, "inspect", "process"),
	)
	byName := map[string]inspectProcess{}
	for _, process := range report.Processes {
		byName[process.Name] = process
	}
	for _, name := range []string{"app", "migrate", "queue", "seeds", "ssr"} {
		process, ok := byName[name]
		if !ok {
			t.Fatalf("missing process %q in %#v", name, report.Processes)
		}
		if process.Source != "cmd/"+name+"/main.go" {
			t.Fatalf("%s source = %q", name, process.Source)
		}
		if !process.ResponsibilityKnown || process.Responsibility == "" {
			t.Fatalf("%s responsibility = %#v", name, process)
		}
	}
	if byName["seeds"].Buildable {
		t.Fatalf("seeds should not be marked buildable: %#v", byName["seeds"])
	}
	if byName["migrate"].StartupOrder != 1 || byName["ssr"].StartupOrder != 2 ||
		byName["app"].StartupOrder != 3 || byName["queue"].StartupOrder != 3 {
		t.Fatalf("startup order = %#v", byName)
	}
	if byName["app"].HealthEndpoint != "/api/health" || !byName["app"].HealthResolved {
		t.Fatalf("app health = %#v", byName["app"])
	}
	if byName["ssr"].HealthEndpoint != "http://127.0.0.1:13714/health" ||
		!byName["ssr"].HealthResolved {
		t.Fatalf("ssr health = %#v", byName["ssr"])
	}
	if byName["queue"].HealthEndpoint != "" || !byName["queue"].HealthResolved {
		t.Fatalf("queue health = %#v", byName["queue"])
	}
	if byName["migrate"].HealthEndpoint != "" || !byName["migrate"].HealthResolved {
		t.Fatalf("migrate health = %#v", byName["migrate"])
	}
}

func TestInspectViewsAreReadOnly(t *testing.T) {
	root := writeInspectViewsProject(t)
	writeTestFile(t, root, ".env", "HOST_PRIMARY=localhost:8080\nPROTOCOL=http\n")
	unsetEnvForTest(t, "HOST_PRIMARY", "HOST_ADMIN", "PROTOCOL", "INERTIA_SSR_LISTEN", "INERTIA_SSR_URL")
	before := snapshotTree(t, root)

	for _, args := range [][]string{
		{"inspect", "pages"},
		{"inspect", "process"},
		{"inspect", "urls"},
	} {
		result := runCLIOnProject(t, root, args...)
		if result.err != nil {
			t.Fatalf("%v: %v\n%s", args, result.err, result.stderr)
		}
	}
	after := snapshotTree(t, root)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("inspection changed the tree\nbefore: %#v\nafter: %#v", before, after)
	}
}

func snapshotTree(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	snapshot := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = sha256.Sum256(body)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree: %v", err)
	}
	return snapshot
}

func TestInspectViewsSupportStructuredModesAndJQ(t *testing.T) {
	root := writeInspectViewsProject(t)
	writeTestFile(t, root, ".env", "HOST_PRIMARY=localhost:8080\nPROTOCOL=http\n")
	unsetEnvForTest(t, "HOST_PRIMARY", "HOST_ADMIN", "PROTOCOL", "INERTIA_SSR_LISTEN", "INERTIA_SSR_URL")

	for _, args := range [][]string{
		{"inspect", "pages", "--agent"},
		{"inspect", "process", "--agent"},
		{"inspect", "urls", "--agent"},
		{"inspect", "pages", "--md"},
		{"inspect", "process", "--md"},
		{"inspect", "urls", "--md"},
	} {
		result := runCLIOnProject(t, root, args...)
		if result.err != nil {
			t.Fatalf("%v: %v\nstderr:\n%s", args, result.err, result.stderr)
		}
	}

	for _, test := range []struct {
		args []string
		key  string
	}{
		{args: []string{"inspect", "pages", "--jq", ".pages"}, key: "component"},
		{args: []string{"inspect", "process", "--jq", ".processes"}, key: "name"},
		{args: []string{"inspect", "urls", "--jq", ".routes"}, key: "name"},
	} {
		result := runCLIOnProject(t, root, test.args...)
		if result.err != nil {
			t.Fatalf("%v: %v\nstderr:\n%s", test.args, result.err, result.stderr)
		}
		var items []map[string]any
		if err := json.Unmarshal([]byte(result.stdout), &items); err != nil {
			t.Fatalf("%v: decode projection: %v\n%s", test.args, err, result.stdout)
		}
		if len(items) == 0 {
			t.Fatalf("%v: empty projection", test.args)
		}
		if _, ok := items[0][test.key]; !ok {
			t.Fatalf("%v: projected item has no %q: %#v", test.args, test.key, items[0])
		}
	}
}

func TestInspectURLsPrefersExportedEnvironmentAndResolvesExtraHosts(t *testing.T) {
	root := writeInspectViewsProject(t)
	writeTestFile(t, root, ".env", "HOST_PRIMARY=file.example:8080\nHOST_ADMIN=file-admin.example\nPROTOCOL=http\nSECRET_TOKEN=do-not-print\n")
	t.Setenv("HOST_PRIMARY", "staging.example.com:8443")
	t.Setenv("HOST_ADMIN", "admin.example.com")
	t.Setenv("PROTOCOL", "https")
	t.Setenv("SECRET_TOKEN", "super-secret-value")

	reportBytes := runInspectJSON(t, root, "inspect", "urls")
	report := decodeInspectEnvelope[inspectURLsReport](t, reportBytes)
	byName := map[string]inspectURLRoute{}
	for _, route := range report.Routes {
		byName[route.Name] = route
	}

	home := byName["pages.home"]
	if !home.Resolved || home.Origin != "https://staging.example.com:8443" {
		t.Fatalf("pages.home = %#v", home)
	}
	if !strings.Contains(home.OriginSource, "exported environment") {
		t.Fatalf("pages.home source = %q", home.OriginSource)
	}

	admin := byName["admin.home"]
	if !admin.Resolved || admin.Origin != "https://admin.example.com" {
		t.Fatalf("admin.home = %#v", admin)
	}
	if admin.HostConfigKey != "HOST_ADMIN" {
		t.Fatalf("admin.home host config key = %q", admin.HostConfigKey)
	}

	if strings.Contains(string(reportBytes), "super-secret-value") ||
		strings.Contains(string(reportBytes), "do-not-print") {
		t.Fatalf("inspect urls leaked an unrelated environment value:\n%s", reportBytes)
	}
}

func TestInspectProcessDoesNotInventUndocumentedProcesses(t *testing.T) {
	root := writeInspectViewsProject(t)
	writeTestFile(t, root, "cmd/custom/main.go", "package main\n")
	writeTestFile(t, root, ".env", "HOST_PRIMARY=localhost:8080\nPROTOCOL=http\n")

	report := decodeInspectEnvelope[inspectProcessReport](
		t,
		runInspectJSON(t, root, "inspect", "process"),
	)
	var custom *inspectProcess
	for i := range report.Processes {
		if report.Processes[i].Name == "custom" {
			custom = &report.Processes[i]
		}
	}
	if custom == nil {
		t.Fatalf("missing custom process in %#v", report.Processes)
	}
	if custom.ResponsibilityKnown || custom.Responsibility != "" ||
		custom.HealthResolved || custom.StartupOrder != 0 ||
		!custom.Buildable {
		t.Fatalf("custom process = %#v", custom)
	}
	if !strings.Contains(custom.StartupNote, "No documented") {
		t.Fatalf("custom startup note = %q", custom.StartupNote)
	}
}

func TestInspectProcessEmptyProject(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	report := decodeInspectEnvelope[inspectProcessReport](
		t,
		runInspectJSON(t, root, "inspect", "process"),
	)
	if len(report.Processes) != 0 {
		t.Fatalf("processes = %#v", report.Processes)
	}
}

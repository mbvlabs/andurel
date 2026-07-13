package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectRouteManifestExtractsStaticRoutes(t *testing.T) {
	rootDir := t.TempDir()
	writeRouteManifestTestFile(t, rootDir, "users.go", `package routes

import "example.com/app/internal/routing"

const UserPrefix = "/users"

var SessionCreate = routing.NewSimpleRoute(
	"/sign-in",
	"users.user_session",
	UserPrefix,
)

var PasswordEdit = routing.NewRouteWithToken(
	"/password/:token/edit",
	"users.edit_user_password",
	UserPrefix,
)
`)
	writeRouteManifestTestFile(t, rootDir, "widgets.go", `package routes

import "example.com/app/internal/routing"

const WidgetPrefix = "/widgets"

var WidgetShow = routing.NewRouteWithUUIDID(
	"/:id",
	"widgets.show",
	WidgetPrefix,
)

var WidgetIndex = routing.NewSimpleRoute(
	"",
	"widgets.index",
	WidgetPrefix,
)
`)

	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		t.Fatalf("collect route manifest: %v", err)
	}

	if len(manifest.Routes) != 4 {
		t.Fatalf("expected 4 routes, got %#v", manifest.Routes)
	}
	if len(manifest.Skipped) != 0 {
		t.Fatalf("expected no skipped routes, got %#v", manifest.Skipped)
	}

	assertRouteManifestRoute(t, manifest, "SessionCreate", "users.user_session", "/users/sign-in", "simple", nil)
	assertRouteManifestRoute(t, manifest, "PasswordEdit", "users.edit_user_password", "/users/password/:token/edit", "token", []routeManifestParam{{Name: "token", Type: "string"}})
	assertRouteManifestRoute(t, manifest, "WidgetShow", "widgets.show", "/widgets/:id", "uuid_id", []routeManifestParam{{Name: "id", Type: "uuid"}})
	assertRouteManifestRoute(t, manifest, "WidgetIndex", "widgets.index", "/widgets", "simple", nil)
}

func TestCollectRouteManifestSupportsConstExpressionsAndGenericParams(t *testing.T) {
	rootDir := t.TempDir()
	writeRouteManifestTestFile(t, rootDir, "admin_dashboards.go", `package routes

import "example.com/app/internal/routing"

const APIPrefix = "/api"
const AdminPrefix = APIPrefix + "/admin"

var DashboardLookup = routing.NewRouteWithParams[DashboardLookupParams](
	"/teams/:team_id/dashboards/:dashboard_id",
	"admin.dashboards.lookup",
	AdminPrefix,
)
`)

	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		t.Fatalf("collect route manifest: %v", err)
	}

	assertRouteManifestRoute(t, manifest, "DashboardLookup", "admin.dashboards.lookup", "/api/admin/teams/:team_id/dashboards/:dashboard_id", "params", []routeManifestParam{
		{Name: "team_id", Type: "string"},
		{Name: "dashboard_id", Type: "string"},
	})
}

func TestCollectRouteManifestSkipsDynamicRoutes(t *testing.T) {
	rootDir := t.TempDir()
	writeRouteManifestTestFile(t, rootDir, "assets.go", `package routes

import (
	"fmt"
	"example.com/app/internal/routing"
)

const AssetsPrefix = "/assets"

var Scripts = routing.NewSimpleRoute(
	fmt.Sprintf("/js/%v/scripts.js", 123),
	"js.scripts",
	AssetsPrefix,
)
`)

	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		t.Fatalf("collect route manifest: %v", err)
	}

	if len(manifest.Routes) != 0 {
		t.Fatalf("expected no routes, got %#v", manifest.Routes)
	}
	if len(manifest.Skipped) != 1 {
		t.Fatalf("expected one skipped route, got %#v", manifest.Skipped)
	}
	skip := manifest.Skipped[0]
	if skip.Variable != "Scripts" || skip.Constructor != "NewSimpleRoute" || skip.Reason == "" {
		t.Fatalf("unexpected skipped route: %#v", skip)
	}
}

func TestRenderRouteManifestHumanIncludesRoutes(t *testing.T) {
	var output bytes.Buffer
	manifest := routeManifest{
		Routes: []routeManifestRoute{{
			Variable:   "SessionCreate",
			Name:       "users.user_session",
			Path:       "/users/sign-in",
			Kind:       "simple",
			SourceFile: "router/routes/users.go",
			Line:       17,
		}},
		Skipped: []routeManifestSkipped{{
			Variable:    "Scripts",
			Constructor: "NewSimpleRoute",
			SourceFile:  "router/routes/assets.go",
			Line:        104,
			Reason:      "route path is not a static string expression",
		}},
	}

	if err := renderRouteManifestHuman(&output, manifest); err != nil {
		t.Fatalf("render route manifest: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"Routes (1)",
		"VARIABLE",
		"SessionCreate",
		"users.user_session",
		"/users/sign-in",
		"Skipped (1)",
		"route path is not a static string expression",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestRouteManifestCoversConstructorsAndSkipReasons(t *testing.T) {
	rootDir := t.TempDir()
	writeRouteManifestTestFile(t, rootDir, "all.go", `package routes

import "example.com/app/internal/routing"

const Root = "/root/"
const Nested = (Root + "nested")
const CycleA = CycleB
const CycleB = CycleA

var UUID = routing.NewRouteWithUUIDID("/:uuid", "uuid", Root)
var Serial = routing.NewRouteWithSerialID("/:serial", "serial", Root)
var BigSerial = routing.NewRouteWithBigSerialID("/:big", "big", Root)
var StringID = routing.NewRouteWithStringID("/:string", "string", Root)
var Slug = routing.NewRouteWithSlug("/:slug", "slug", Root)
var Token = routing.NewRouteWithToken("/:token", "token", Root)
var File = routing.NewRouteWithFile("/:file", "file", Root)
var Slugs = routing.NewRouteWithSlugs[any]("/:one/:two", "slugs", Nested)
var TooFew = routing.NewSimpleRoute("/missing")
var DynamicName = routing.NewSimpleRoute("/name", makeName(), Root)
var DynamicPrefix = routing.NewSimpleRoute("/prefix", "prefix", CycleA)
var NotRouting = other.NewSimpleRoute("/ignored", "ignored", Root)
var NotCall = Root
`)

	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		t.Fatalf("collect route manifest: %v", err)
	}
	if len(manifest.Routes) != 8 {
		t.Fatalf("expected eight routes, got %#v", manifest.Routes)
	}
	if len(manifest.Skipped) != 3 {
		t.Fatalf("expected three skipped routes, got %#v", manifest.Skipped)
	}

	wants := map[string]struct {
		kind      string
		paramType string
	}{
		"UUID":      {kind: "uuid_id", paramType: "uuid"},
		"Serial":    {kind: "serial_id", paramType: "int32"},
		"BigSerial": {kind: "bigserial_id", paramType: "int64"},
		"StringID":  {kind: "string_id", paramType: "string"},
		"Slug":      {kind: "slug", paramType: "string"},
		"Token":     {kind: "token", paramType: "string"},
		"File":      {kind: "file", paramType: "string"},
		"Slugs":     {kind: "params", paramType: "string"},
	}
	for variable, want := range wants {
		route, ok := findRouteManifestRoute(manifest, variable)
		if !ok {
			t.Fatalf("missing route %s in %#v", variable, manifest.Routes)
		}
		if route.Kind != want.kind || len(route.Params) == 0 || route.Params[0].Type != want.paramType {
			t.Fatalf("unexpected route %s: %#v", variable, route)
		}
	}
	for _, skipped := range manifest.Skipped {
		if skipped.Reason == "" || skipped.Line == 0 || skipped.SourceFile != "router/routes/all.go" {
			t.Fatalf("incomplete skipped route: %#v", skipped)
		}
	}
}

func TestRouteManifestHelpersAndFilesystemErrors(t *testing.T) {
	if manifest, err := collectRouteManifest(t.TempDir()); err != nil || len(manifest.Routes) != 0 {
		t.Fatalf("missing routes directory should be empty: %#v, %v", manifest, err)
	}

	rootDir := t.TempDir()
	routesPath := filepath.Join(rootDir, "router", "routes")
	if err := os.MkdirAll(filepath.Dir(routesPath), 0o755); err != nil {
		t.Fatalf("create router directory: %v", err)
	}
	if err := os.WriteFile(routesPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write routes file: %v", err)
	}
	if _, err := collectRouteManifest(rootDir); err == nil {
		t.Fatal("expected routes path read error")
	}

	invalidDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidDir, "bad.go"), []byte("package routes\nvar"), 0o644); err != nil {
		t.Fatalf("write invalid route file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "README.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(invalidDir, "nested.go"), 0o755); err != nil {
		t.Fatalf("create ignored directory: %v", err)
	}
	if _, err := parseRouteFiles(invalidDir); err == nil || !strings.Contains(err.Error(), "parse route file") {
		t.Fatalf("expected parse error, got %v", err)
	}

	paths := map[string]string{
		"":                     "/",
		"users":                "/users",
		" /users//active/?x#y": "/users/active",
		`\users\active`:        "/users/active",
	}
	for input, want := range paths {
		if got := configureRouteManifestPath(input, ""); got != want {
			t.Fatalf("configure path %q = %q, want %q", input, got, want)
		}
	}
	joins := []struct{ path, prefix, want string }{
		{"", "/api", "/api"},
		{"users", "/api/", "/api/users"},
		{"/users", "/api/", "/api/users"},
		{"users", "/api", "/api/users"},
	}
	for _, test := range joins {
		if got := configureRouteManifestPath(test.path, test.prefix); got != test.want {
			t.Fatalf("configure path %q + %q = %q, want %q", test.prefix, test.path, got, test.want)
		}
	}

	if got := routeKind("unknown"); got != "unknown" {
		t.Fatalf("unknown route kind = %q", got)
	}
	if got := formatRouteManifestParams(nil); got != "-" {
		t.Fatalf("empty params = %q", got)
	}
	if got := formatRouteManifestParams([]routeManifestParam{{Name: "id", Type: "uuid"}, {Name: "slug", Type: "string"}}); got != "id:uuid,slug:string" {
		t.Fatalf("formatted params = %q", got)
	}

	var output bytes.Buffer
	if err := renderRouteManifestHuman(&output, routeManifest{}); err != nil || output.String() != "No routes found.\n" {
		t.Fatalf("empty manifest output = %q, %v", output.String(), err)
	}
	if err := renderRouteManifestHuman(routeManifestFailWriter{}, routeManifest{}); !errors.Is(err, errRouteManifestWrite) {
		t.Fatalf("expected writer error, got %v", err)
	}
}

var errRouteManifestWrite = errors.New("route manifest write failed")

type routeManifestFailWriter struct{}

func (routeManifestFailWriter) Write([]byte) (int, error) {
	return 0, errRouteManifestWrite
}

func writeRouteManifestTestFile(t *testing.T, rootDir, filename, content string) {
	t.Helper()

	routesDir := filepath.Join(rootDir, "router", "routes")
	if err := os.MkdirAll(routesDir, 0o755); err != nil {
		t.Fatalf("create routes dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(routesDir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write route file: %v", err)
	}
}

func assertRouteManifestRoute(
	t *testing.T,
	manifest routeManifest,
	variable string,
	name string,
	path string,
	kind string,
	params []routeManifestParam,
) {
	t.Helper()

	route, ok := findRouteManifestRoute(manifest, variable)
	if !ok {
		t.Fatalf("expected route %s in %#v", variable, manifest.Routes)
	}
	if route.Name != name || route.Path != path || route.Kind != kind {
		t.Fatalf("unexpected route %s: %#v", variable, route)
	}
	if len(route.Params) != len(params) {
		t.Fatalf("expected params %#v for %s, got %#v", params, variable, route.Params)
	}
	for i := range params {
		if route.Params[i] != params[i] {
			t.Fatalf("expected param %#v for %s at %d, got %#v", params[i], variable, i, route.Params[i])
		}
	}
}

func findRouteManifestRoute(manifest routeManifest, variable string) (routeManifestRoute, bool) {
	for _, route := range manifest.Routes {
		if route.Variable == variable {
			return route, true
		}
	}
	return routeManifestRoute{}, false
}

package cli

import (
	"bytes"
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

package routing

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// captureWarnings routes the default slog logger into a buffer for the
// duration of the test and returns the buffer.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func TestIndexUnderPrefixDoesNotWarnAboutTrailingSlash(t *testing.T) {
	logs := captureWarnings(t)

	route := NewSimpleRoute("/", "index", "/users")
	if got, want := route.Path(), "/users"; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if strings.Contains(logs.String(), "trailing slash") {
		t.Fatalf("unexpected trailing slash warning: %s", logs.String())
	}
}

func TestTrailingSlashStillWarnsForOtherPaths(t *testing.T) {
	logs := captureWarnings(t)

	route := NewSimpleRoute("/articles/", "articles.index", "")
	if got, want := route.Path(), "/articles"; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if !strings.Contains(logs.String(), "path has trailing slash") {
		t.Fatalf("expected trailing slash warning, got: %q", logs.String())
	}
}

func TestIsInertiaDefaultsFalse(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "")
	if route.IsInertia() {
		t.Fatal("expected IsInertia to default to false")
	}

	uuidRoute := NewRouteWithUUIDID("/:id", "widgets.show", "/widgets")
	if uuidRoute.IsInertia() {
		t.Fatal("expected IsInertia to default to false for UUID routes")
	}
}

func TestTypeScriptRouteOption(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "", TypeScriptRoute())
	if !route.IsInertia() {
		t.Fatal("expected TypeScriptRoute option to enable the flag")
	}

	paramsRoute := NewRouteWithParams[any]("/:slug", "posts.show", "/posts", TypeScriptRoute())
	if !paramsRoute.IsInertia() {
		t.Fatal("expected TypeScriptRoute option to enable the flag for params routes")
	}
}

func TestInertiaRouteIsDeprecatedAlias(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "", InertiaRoute())
	if !route.IsInertia() {
		t.Fatal("expected deprecated InertiaRoute alias to enable the same flag")
	}
}

func TestExactRoutePattern(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "", Exact())
	if got, want := route.Path(), "/"; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if got, want := route.Pattern(), "/{$}"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}

	// Non-root paths are already exact in net/http, so Pattern is the path.
	about := NewSimpleRoute("/about", "pages.about", "", Exact())
	if got, want := about.Pattern(), "/about"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}
}

func TestRootRouteIsExactWithoutOption(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "")
	if got, want := route.Pattern(), "/{$}"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}

	// A "/" route under a prefix is the prefix's index, which is exact too.
	index := NewSimpleRoute("/", "index", "/users")
	if got, want := index.Path(), "/users"; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if got, want := index.Pattern(), "/users"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}
}

func TestCatchAllRouteIsSubtree(t *testing.T) {
	route := NewSimpleRoute("/*", "static", "")
	if got, want := route.Pattern(), "/*"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}
	if got, want := route.Prefix(), "/"; got != want {
		t.Fatalf("Prefix() = %q, want %q", got, want)
	}
	if !route.InSubtree("/anything/at/all") {
		t.Fatal("expected /* to cover every path")
	}
}

func TestWildcardParamName(t *testing.T) {
	if WildcardParam != "path" {
		t.Fatalf("WildcardParam = %q, want %q", WildcardParam, "path")
	}
}

func TestRouteNameExcludesPrefix(t *testing.T) {
	cases := []struct {
		name string
		got  string
	}{
		{"simple", NewSimpleRoute("/new", "new_session", "/users").Name()},
		{"uuid", NewRouteWithUUIDID("/:id", "show", "/users").Name()},
		{"serial", NewRouteWithSerialID("/:id", "show", "/users").Name()},
		{"bigserial", NewRouteWithBigSerialID("/:id", "show", "/users").Name()},
		{"string", NewRouteWithStringID("/:id", "show", "/users").Name()},
		{"slug", NewRouteWithSlug("/:slug", "show", "/users").Name()},
		{"token", NewRouteWithToken("/:token", "confirm", "/users").Name()},
		{"file", NewRouteWithFile("/:file", "get", "/assets").Name()},
		{"params", NewRouteWithParams[any]("/:slug", "show", "/posts").Name()},
	}
	want := map[string]string{
		"simple":    "new_session",
		"uuid":      "show",
		"serial":    "show",
		"bigserial": "show",
		"string":    "show",
		"slug":      "show",
		"token":     "confirm",
		"file":      "get",
		"params":    "show",
	}
	for _, tc := range cases {
		if tc.got != want[tc.name] {
			t.Fatalf("%s Name() = %q, want %q", tc.name, tc.got, want[tc.name])
		}
	}
}

func TestPatternDefaultsToPath(t *testing.T) {
	route := NewSimpleRoute("/users", "users.index", "")
	if got, want := route.Pattern(), route.Path(); got != want {
		t.Fatalf("Pattern() = %q, want Path() %q", got, want)
	}

	uuidRoute := NewRouteWithUUIDID("/:id", "users.show", "/users")
	if got, want := uuidRoute.Pattern(), "/users/:id"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}

	paramsRoute := NewRouteWithParams[any]("/:slug", "posts.show", "/posts")
	if got, want := paramsRoute.Pattern(), "/posts/:slug"; got != want {
		t.Fatalf("Pattern() = %q, want %q", got, want)
	}
}

func TestRoutePrefixAndSubtree(t *testing.T) {
	vite := NewSimpleRoute("*", "vite.build", "/assets/dist")
	if got, want := vite.Prefix(), "/assets/dist"; got != want {
		t.Fatalf("Prefix() = %q, want %q", got, want)
	}
	for _, path := range []string{"/assets/dist", "/assets/dist/app.js", "/assets/dist/nested/chunk.css"} {
		if !vite.InSubtree(path) {
			t.Fatalf("InSubtree(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"/", "/assets", "/assets/district/app.js", "/assets/distx"} {
		if vite.InSubtree(path) {
			t.Fatalf("InSubtree(%q) = true, want false", path)
		}
	}

	rootWildcard := NewSimpleRoute("*", "assets.all", "/")
	if got, want := rootWildcard.Prefix(), "/"; got != want {
		t.Fatalf("root wildcard Prefix() = %q, want %q", got, want)
	}
	if !rootWildcard.InSubtree("/anything") || rootWildcard.InSubtree("relative") {
		t.Fatal("root wildcard InSubtree should match absolute paths only")
	}

	users := NewSimpleRoute("/users", "users.index", "")
	if got, want := users.Prefix(), "/users"; got != want {
		t.Fatalf("non-wildcard Prefix() = %q, want %q", got, want)
	}
	if !users.InSubtree("/users") || !users.InSubtree("/users/1") || users.InSubtree("/usersx") {
		t.Fatal("non-wildcard InSubtree does not match its path subtree")
	}
}

func TestHostDefaultsToPrimary(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "")
	if route.Host() != HostPrimary {
		t.Fatalf("expected HostPrimary, got %q", route.Host())
	}
}

func TestHostOption(t *testing.T) {
	const hostAdmin HostName = "admin"
	route := NewSimpleRoute("/widgets", "widgets.index", "", Host(hostAdmin))
	if route.Host() != hostAdmin {
		t.Fatalf("expected %q, got %q", hostAdmin, route.Host())
	}
}

func TestNewHostsRequiresPrimary(t *testing.T) {
	if _, err := NewHosts(nil); err == nil {
		t.Fatal("expected error for empty registry")
	}
	if _, err := NewHosts(map[HostName]HostSpec{
		"admin": {Hostname: "admin.example.com", Protocol: "https"},
	}); err == nil {
		t.Fatal("expected error when primary is missing")
	}
	if _, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "", Protocol: "https"},
	}); err == nil {
		t.Fatal("expected error when primary hostname is empty")
	}
}

func TestFullURLUsesHosts(t *testing.T) {
	const hostAdmin HostName = "admin"
	hosts, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:   {Hostname: "admin.andurel.com", Protocol: "https"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	primary := NewSimpleRoute("/", "pages.home", "")
	if got, want := primary.FullURL(hosts), "https://andurel.com/"; got != want {
		t.Fatalf("primary FullURL = %q, want %q", got, want)
	}

	admin := NewSimpleRoute("", "widgets.index", "/widgets", Host(hostAdmin))
	if got, want := admin.FullURL(hosts), "https://admin.andurel.com/widgets"; got != want {
		t.Fatalf("admin FullURL = %q, want %q", got, want)
	}

	if got, want := admin.FullURL(nil), "/widgets"; got != want {
		t.Fatalf("nil hosts FullURL = %q, want %q", got, want)
	}
}

func TestHostsBaseURLUnknownReturnsEmpty(t *testing.T) {
	hosts, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	if got := hosts.BaseURL("admin"); got != "" {
		t.Fatalf("unknown host BaseURL = %q, want empty", got)
	}
}

func TestNewHostsRejectsHostnameCollision(t *testing.T) {
	_, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		"admin":     {Hostname: "andurel.com", Protocol: "https"},
	})
	if err == nil {
		t.Fatal("expected hostname collision error")
	}
	if !strings.Contains(err.Error(), "andurel.com") {
		t.Fatalf("error should name the hostname, got %v", err)
	}
}

func TestNewHostsRejectsAliasCollision(t *testing.T) {
	_, err := NewHosts(map[HostName]HostSpec{
		HostPrimary: {Hostname: "andurel.com", Aliases: []string{"www.andurel.com"}, Protocol: "https"},
		"admin":     {Hostname: "admin.andurel.com", Aliases: []string{"www.andurel.com"}, Protocol: "https"},
	})
	if err == nil {
		t.Fatal("expected alias collision error")
	}
}

func TestHostSpecOriginsIncludesAliases(t *testing.T) {
	spec := HostSpec{
		Hostname: "andurel.com",
		Aliases:  []string{"www.andurel.com", "andurel.com"},
		Protocol: "https",
	}
	origins := spec.Origins()
	if len(origins) != 2 {
		t.Fatalf("expected 2 unique origins, got %#v", origins)
	}
	if origins[0] != "https://andurel.com" || origins[1] != "https://www.andurel.com" {
		t.Fatalf("unexpected origins: %#v", origins)
	}
}

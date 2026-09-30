package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/mbvlabs/andurel/pkg/routing"
)

type testHostedRoute struct {
	name string
	path string
	host routing.HostName
}

func (r testHostedRoute) Name() string           { return r.name }
func (r testHostedRoute) Path() string           { return r.path }
func (r testHostedRoute) Host() routing.HostName { return r.host }

func TestEchoForUnknownHost(t *testing.T) {
	r := &Router{
		hosts: map[routing.HostName]*echo.Echo{
			routing.HostPrimary: echo.New(),
		},
	}

	if _, err := r.echoFor("admin"); err == nil {
		t.Fatal("expected unconfigured host to fail")
	}
}

func TestAddRouteFillsPathNameAndDispatches(t *testing.T) {
	primary := echo.New()
	admin := echo.New()
	r := &Router{
		hosts: map[routing.HostName]*echo.Echo{
			routing.HostPrimary: primary,
			"admin":             admin,
		},
	}

	rt := testHostedRoute{name: "widgets.index", path: "/widgets", host: "admin"}
	info, err := r.AddRoute(rt, echo.Route{
		Method:  http.MethodGet,
		Handler: func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) },
	})
	if err != nil {
		t.Fatalf("AddRoute: %v", err)
	}
	if info.Path != "/widgets" || info.Name != "widgets.index" {
		t.Fatalf("route info = %+v", info)
	}
	if len(admin.Router().Routes()) != 1 {
		t.Fatalf("admin routes = %d, want 1", len(admin.Router().Routes()))
	}
	if len(primary.Router().Routes()) != 0 {
		t.Fatalf("primary should not receive the admin route")
	}
}

func TestAddRouteUnknownHostNamesTheRoute(t *testing.T) {
	r := &Router{
		hosts: map[routing.HostName]*echo.Echo{
			routing.HostPrimary: echo.New(),
		},
	}

	_, err := r.AddRoute(
		testHostedRoute{name: "widgets.index", path: "/widgets", host: "admin"},
		echo.Route{Method: http.MethodGet, Handler: func(c *echo.Context) error { return nil }},
	)
	if err == nil {
		t.Fatal("expected unconfigured host error")
	}
	if !strings.Contains(err.Error(), "widgets.index") {
		t.Fatalf("error should name the route, got %v", err)
	}
}

func TestCORSDefaultsToApplicationOrigin(t *testing.T) {
	config, err := newCORSConfig([]string{"https://app.example.com"}, nil)
	if err != nil {
		t.Fatalf("newCORSConfig returned an error: %v", err)
	}

	want := []string{"https://app.example.com"}
	if !reflect.DeepEqual(config.AllowOrigins, want) {
		t.Fatalf("AllowOrigins = %v, want %v", config.AllowOrigins, want)
	}
	if !config.AllowCredentials {
		t.Fatal("expected credentialed CORS")
	}
}

func TestCORSRequiresExplicitAdditionalOrigins(t *testing.T) {
	config, err := newCORSConfig(
		[]string{"https://app.example.com"},
		[]string{" https://admin.example.com ", ""},
	)
	if err != nil {
		t.Fatalf("newCORSConfig returned an error: %v", err)
	}

	want := []string{"https://app.example.com", "https://admin.example.com"}
	if !reflect.DeepEqual(config.AllowOrigins, want) {
		t.Fatalf("AllowOrigins = %v, want %v", config.AllowOrigins, want)
	}
}

func TestCredentialedCORSRejectsWildcards(t *testing.T) {
	tests := []struct {
		name               string
		applicationOrigins []string
		additionalOrigins  []string
	}{
		{name: "wildcard application origin", applicationOrigins: []string{"*"}},
		{
			name:               "wildcard additional origin",
			applicationOrigins: []string{"https://app.example.com"},
			additionalOrigins:  []string{"https://*"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newCORSConfig(test.applicationOrigins, test.additionalOrigins); err == nil {
				t.Fatalf("expected wildcard CORS configuration to be rejected")
			}
		})
	}
}

func TestBindHostnameRejectsCollision(t *testing.T) {
	vhosts := map[string]*echo.Echo{}
	first := echo.New()
	second := echo.New()
	if err := bindHostname(vhosts, "admin.example.com", first); err != nil {
		t.Fatalf("first bind: %v", err)
	}
	if err := bindHostname(vhosts, "admin.example.com", second); err == nil {
		t.Fatal("expected hostname collision")
	}
	if err := bindHostname(vhosts, "admin.example.com:443", second); err == nil {
		t.Fatal("expected ported hostname collision")
	}
}

func TestBindHostnameRegistersStandardPorts(t *testing.T) {
	vhosts := map[string]*echo.Echo{}
	child := echo.New()
	if err := bindHostname(vhosts, "admin.example.com", child); err != nil {
		t.Fatalf("bind: %v", err)
	}
	for _, key := range []string{"admin.example.com", "admin.example.com:80", "admin.example.com:443"} {
		if vhosts[key] != child {
			t.Fatalf("missing binding for %q", key)
		}
	}
}

func TestVirtualHostDispatchesByHostHeader(t *testing.T) {
	admin := echo.New()
	admin.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    "/",
		Handler: func(c *echo.Context) error { return c.String(http.StatusOK, "admin") },
	})

	vhosts := map[string]*echo.Echo{}
	if err := bindHostname(vhosts, "admin.example.com", admin); err != nil {
		t.Fatalf("bind: %v", err)
	}

	primary := echo.NewVirtualHostHandler(vhosts)
	primary.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    "/",
		Handler: func(c *echo.Context) error { return c.String(http.StatusOK, "primary") },
	})

	assertBody := func(t *testing.T, host, want string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		primary.ServeHTTP(rec, req)
		body, err := io.ReadAll(rec.Result().Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if got := string(body); got != want {
			t.Fatalf("Host %q body = %q, want %q", host, got, want)
		}
	}

	assertBody(t, "admin.example.com", "admin")
	assertBody(t, "admin.example.com:443", "admin")
	assertBody(t, "unknown.example.com", "primary")
}

func TestAddRouteNotFoundEachHost(t *testing.T) {
	primary := echo.New()
	admin := echo.New()
	r := &Router{
		hosts: map[routing.HostName]*echo.Echo{
			routing.HostPrimary: primary,
			"admin":             admin,
		},
	}

	handler := func(c *echo.Context) error {
		return c.String(http.StatusNotFound, "missing")
	}
	if err := r.AddRouteNotFoundEachHost(handler); err != nil {
		t.Fatalf("AddRouteNotFoundEachHost: %v", err)
	}
}

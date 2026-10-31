package anduin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/routing"
)

func TestMuxPattern(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/users":           "/users",
		"/users/:id":       "/users/{id}",
		"/tokens/:token":   "/tokens/{token}",
		"/files/*":         "/files/{path...}",
		"/a/:b/c/:d":       "/a/{b}/c/{d}",
		"/already/{value}": "/already/{value}",
		"/":                "/",
	}
	for input, want := range cases {
		if got := MuxPattern(input); got != want {
			t.Errorf("MuxPattern(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRouterDispatchesAndReadsPathValue(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/users/:id", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "id="+PathValue(ctx, "id"))
	})

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users/42", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Body.String(); got != "id=42" {
		t.Fatalf("body = %q, want %q", got, "id=42")
	}
}

func TestRouterRunsPerRouteMiddleware(t *testing.T) {
	t.Parallel()

	var order []string
	mark := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context) error {
				order = append(order, name)
				return next(ctx)
			}
		}
	}

	rt := NewRouter()
	rt.Use(mark("global"))
	rt.Get("/", func(ctx context.Context) error {
		order = append(order, "handler")
		return NoContent(ctx, http.StatusNoContent)
	}, mark("route"))

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"global", "route", "handler"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestRouterErrorHandler(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		if !errors.Is(err, sentinel) {
			t.Errorf("err = %v, want sentinel", err)
		}
		return Text(ctx, http.StatusTeapot, "handled")
	}))
	rt.Get("/", func(context.Context) error { return sentinel })

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", recorder.Code)
	}
	if got := recorder.Body.String(); got != "handled" {
		t.Fatalf("body = %q, want handled", got)
	}
}

func TestRouterDefaultErrorStatus(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/", func(ctx context.Context) error { return NotFound(errors.New("nope")) })

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestRouterUnmatchedRequestRoutesToErrorHandler(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		return Text(ctx, http.StatusNotFound, "themed not found")
	}))
	home := routing.NewSimpleRoute("/", "pages.home", "", routing.Exact(), routing.InertiaRoute())
	rt.Get(home.Pattern(), func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "home")
	})

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "home" {
		t.Fatalf("GET / = %d %q, want 200 home", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got := recorder.Body.String(); got != "themed not found" {
		t.Fatalf("body = %q, want themed not found", got)
	}
}

func TestRouterUnmatchedRequestDefaultHandlerIsNotFound(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got, want := strings.TrimSpace(recorder.Body.String()), http.StatusText(http.StatusNotFound); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestRouterGetServesHead(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/health", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodHead, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestRouterMethodNotAllowedIsPreserved(t *testing.T) {
	t.Parallel()

	var gotErr error
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		gotErr = err
		return Text(ctx, http.StatusMethodNotAllowed, "themed method not allowed")
	}))
	rt.Get("/only-get", func(ctx context.Context) error { return NoContent(ctx, http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/only-get", nil))

	if !errors.Is(gotErr, ErrMethodNotAllowed) {
		t.Fatalf("ErrorHandler err = %v, want ErrMethodNotAllowed", gotErr)
	}
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
	if got, want := recorder.Header().Get("Allow"), "GET, HEAD"; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
	if got, want := recorder.Body.String(), "themed method not allowed"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestRouterMethodNotAllowedIsNeverNotFound(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		if errors.Is(err, ErrNotFound) {
			t.Errorf("405 error %v must not match ErrNotFound", err)
		}
		if !errors.Is(err, ErrMethodNotAllowed) {
			t.Errorf("405 error = %v, want ErrMethodNotAllowed", err)
		}
		return Text(ctx, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
	}))
	rt.Get("/only-get", func(ctx context.Context) error { return NoContent(ctx, http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/only-get", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}

func TestRouterMethodNotAllowedDefaultHandler(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/only-get", func(ctx context.Context) error { return NoContent(ctx, http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/only-get", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
	if got, want := recorder.Header().Get("Allow"), "GET, HEAD"; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestRouterQueryHelpers(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/", func(ctx context.Context) error {
		page, err := QueryInt(ctx, "page", 1)
		if err != nil {
			return BadRequest(err)
		}
		return Text(ctx, http.StatusOK, Query(ctx, "name")+"-"+strconv.Itoa(page))
	})

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/?name=ada&page=3", nil))
	if got := recorder.Body.String(); got != "ada-3" {
		t.Fatalf("body = %q, want ada-3", got)
	}

	bad := httptest.NewRecorder()
	rt.Handler().ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/?page=x", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", bad.Code)
	}
}

func TestRouterHostDispatch(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/", func(ctx context.Context) error { return Text(ctx, http.StatusOK, "primary") })

	// A named host without a configured hostname still dispatches when the
	// request Host matches a configured alias.
	host := rt.Host("admin")
	host.Isolate()
	host.Get("/", func(ctx context.Context) error { return Text(ctx, http.StatusOK, "admin") })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "localhost:8080"
	rt.Handler().ServeHTTP(recorder, request)
	if got := recorder.Body.String(); got != "primary" {
		t.Fatalf("primary body = %q, want primary", got)
	}
}

func TestRouterHostDispatchWithHosts(t *testing.T) {
	t.Parallel()

	const hostAdmin routing.HostName = "admin"
	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:           {Hostname: "admin.andurel.com", Aliases: []string{"manage.andurel.com"}},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	rt := NewRouter(WithHosts(hosts))
	rt.Get("/", func(ctx context.Context) error { return Text(ctx, http.StatusOK, "primary") })
	rt.Host(hostAdmin).Isolate()
	rt.Host(hostAdmin).Get("/", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "admin")
	})

	for _, host := range []string{
		"admin.andurel.com",
		"manage.andurel.com:443",
		"ADMIN.ANDUREL.COM",
		"Manage.Andurel.Com:443",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Host = host
		rt.Handler().ServeHTTP(recorder, request)
		if got := recorder.Body.String(); got != "admin" {
			t.Fatalf("host %q body = %q, want admin", host, got)
		}
	}
}

func TestRouterRequestHostInGlobalMiddleware(t *testing.T) {
	t.Parallel()

	const hostAdmin routing.HostName = "admin"
	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:           {Hostname: "admin.andurel.com"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	var seen []routing.HostName
	rt := NewRouter(WithHosts(hosts))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			seen = append(seen, RequestHost(ctx))
			return next(ctx)
		}
	})
	rt.Get("/", func(ctx context.Context) error { return Text(ctx, http.StatusOK, "primary") })
	rt.Host(hostAdmin).Isolate()
	rt.Host(hostAdmin).Get("/admin", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "admin")
	})

	cases := []struct {
		label  string
		method string
		host   string
		path   string
		status int
		want   routing.HostName
	}{
		{"admin route", http.MethodGet, "admin.andurel.com", "/admin", http.StatusOK, hostAdmin},
		{"admin 404", http.MethodGet, "admin.andurel.com", "/missing", http.StatusNotFound, hostAdmin},
		{"admin 405", http.MethodPost, "admin.andurel.com", "/admin", http.StatusMethodNotAllowed, hostAdmin},
		{"primary route", http.MethodGet, "andurel.com", "/", http.StatusOK, routing.HostPrimary},
		{"unknown host", http.MethodGet, "unconfigured.example", "/", http.StatusOK, routing.HostPrimary},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			seen = seen[:0]

			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Host = tc.host
			recorder := httptest.NewRecorder()
			rt.Handler().ServeHTTP(recorder, request)

			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}
			if len(seen) != 1 || seen[0] != tc.want {
				t.Fatalf("middleware RequestHost = %v, want [%q]", seen, tc.want)
			}
		})
	}
}

func TestRouterRequestHostReportsHostWithoutRoutes(t *testing.T) {
	t.Parallel()

	const hostAdmin routing.HostName = "admin"
	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:           {Hostname: "admin.andurel.com"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	var seen routing.HostName
	rt := NewRouter(WithHosts(hosts))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			seen = RequestHost(ctx)
			return next(ctx)
		}
	})
	rt.Get("/", func(ctx context.Context) error { return Text(ctx, http.StatusOK, "primary") })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "admin.andurel.com"
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, request)

	if seen != hostAdmin {
		t.Fatalf("middleware RequestHost = %q, want %q", seen, hostAdmin)
	}
	if recorder.Code != http.StatusOK || recorder.Body.String() != "primary" {
		t.Fatalf("dispatch = %d %q, want 200 primary", recorder.Code, recorder.Body.String())
	}
}

func TestRequestHostWithoutRouterState(t *testing.T) {
	t.Parallel()

	if got := RequestHost(context.Background()); got != routing.HostPrimary {
		t.Fatalf("RequestHost(background) = %q, want %q", got, routing.HostPrimary)
	}
	if got := RequestHost(nil); got != routing.HostPrimary {
		t.Fatalf("RequestHost(nil) = %q, want %q", got, routing.HostPrimary)
	}
}

func TestCSRFRejectsMissingToken(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Use(CSRF(CSRFConfig{TokenLookup: "header:X-CSRF-Token,cookie:_csrf"}))
	rt.Post("/", func(ctx context.Context) error { return NoContent(ctx, http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestCSRFIssuesTokenOnSafeRequest(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Use(CSRF(CSRFConfig{TokenLookup: "header:X-CSRF-Token,cookie:_csrf"}))
	rt.Get("/", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, CSRFToken(ctx))
	})

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("expected a CSRF token in the body")
	}
	if len(recorder.Result().Cookies()) == 0 {
		t.Fatal("expected a CSRF cookie")
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Use(CORS(CORSConfig{AllowOrigins: []string{"https://app.example"}, AllowCredentials: true}))
	rt.Get("/", func(ctx context.Context) error { return NoContent(ctx, http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://app.example")
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("allow-origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow-credentials = %q", got)
	}
}

func TestServerTimeouts(t *testing.T) {
	t.Parallel()

	defaultServer := NewServer(t.Context(), "localhost", "8080", nil)
	if defaultServer.srv.ReadTimeout <= 0 || defaultServer.srv.WriteTimeout <= 0 ||
		defaultServer.srv.IdleTimeout <= 0 {
		t.Fatal("default server must have bounded HTTP timeouts")
	}

	zeroServer := NewServer(
		t.Context(), "localhost", "8080", nil, WithTimeouts(0, 0, 0),
	)
	if zeroServer.srv.ReadTimeout != 0 || zeroServer.srv.WriteTimeout != 0 ||
		zeroServer.srv.IdleTimeout != 0 {
		t.Fatal("explicit zero timeouts must disable the bounds")
	}
}

const hostAdmin routing.HostName = "admin"

func newAdminHosts(t *testing.T) *routing.Hosts {
	t.Helper()

	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "andurel.com", Protocol: "https"},
		hostAdmin:           {Hostname: "admin.andurel.com"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}
	return hosts
}

func serveOnHost(rt *Router, method, host, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Host = host
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, request)
	return recorder
}

func textHandler(body string) Handler {
	return func(ctx context.Context) error { return Text(ctx, http.StatusOK, body) }
}

func markMiddleware(order *[]string, name string) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			*order = append(*order, name)
			return next(ctx)
		}
	}
}

func TestRouteRegistersOnDeclaredHost(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Get("/dashboard", textHandler("primary"))
	rt.Host(hostAdmin).Isolate()

	dashboard := routing.NewSimpleRoute("/dashboard", "admin.dashboard", "", routing.Host(hostAdmin))
	rt.Route(dashboard).Get(textHandler("admin"))

	admin := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/dashboard")
	if admin.Code != http.StatusOK || admin.Body.String() != "admin" {
		t.Fatalf("admin GET /dashboard = %d %q, want 200 admin", admin.Code, admin.Body.String())
	}

	primary := serveOnHost(rt, http.MethodGet, "andurel.com", "/dashboard")
	if primary.Body.String() != "primary" {
		t.Fatalf("primary GET /dashboard = %d %q, want primary handler", primary.Code, primary.Body.String())
	}
}

func TestRouteOnlyReachableOnDeclaredHost(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Host(hostAdmin).Isolate()
	adminOnly := routing.NewSimpleRoute("/users", "admin.users", "", routing.Host(hostAdmin))
	rt.Route(adminOnly).Get(textHandler("admin users"))

	if got := serveOnHost(rt, http.MethodGet, "andurel.com", "/users"); got.Code != http.StatusNotFound {
		t.Fatalf("primary GET /users = %d, want 404", got.Code)
	}
}

func TestRouteRegistrarRegistersEveryMethod(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Host(hostAdmin).Isolate()
	route := routing.NewSimpleRoute("/items", "admin.items", "", routing.Host(hostAdmin))
	registrar := rt.Route(route)
	registrar.Get(textHandler(http.MethodGet))
	registrar.Post(textHandler(http.MethodPost))
	registrar.Put(textHandler(http.MethodPut))
	registrar.Patch(textHandler(http.MethodPatch))
	registrar.Delete(textHandler(http.MethodDelete))

	for _, method := range []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		recorder := serveOnHost(rt, method, "admin.andurel.com", "/items")
		if recorder.Code != http.StatusOK || recorder.Body.String() != method {
			t.Fatalf("%s /items = %d %q, want 200 %s", method, recorder.Code, recorder.Body.String(), method)
		}
	}
}

func TestHostRouteRejectsRouteDeclaredOnAnotherHost(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	adminRoute := routing.NewSimpleRoute("/users", "admin.users", "", routing.Host(hostAdmin))

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("Host(primary).Route(admin route) did not panic")
		}
		if msg := fmt.Sprint(recovered); !strings.Contains(msg, `declares host "admin"`) {
			t.Fatalf("panic = %q, want a message naming the declared host", msg)
		}
	}()
	rt.Host(routing.HostPrimary).Route(adminRoute)
}

func TestHandlerPanicsWithoutFallbackPolicy(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Host(hostAdmin).Get("/{$}", textHandler("admin"))

	defer func() {
		recovered := recover()
		want := `anduin: host "admin" has routes but no fallback policy; ` +
			`call rt.Host("admin").FallbackToPrimary() or .Isolate()`
		if recovered != want {
			t.Fatalf("panic = %v, want %q", recovered, want)
		}
	}()
	rt.Handler()
}

func TestHostWithoutRoutesNeedsNoPolicy(t *testing.T) {
	t.Parallel()

	var order []string
	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Get("/", textHandler("primary"))
	rt.Host(hostAdmin).Use(markMiddleware(&order, "host"))

	recorder := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/")
	if recorder.Code != http.StatusOK || recorder.Body.String() != "primary" {
		t.Fatalf("admin GET / = %d %q, want 200 primary", recorder.Code, recorder.Body.String())
	}
	if len(order) != 0 {
		t.Fatalf("host middleware ran on a fallback to primary: %v", order)
	}
}

func TestFallbackToPrimaryServesPrimaryRoutes(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Get("/assets/app.js", textHandler("primary asset"))
	rt.Host(hostAdmin).FallbackToPrimary()
	rt.Host(hostAdmin).Get("/{$}", textHandler("admin home"))

	asset := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/assets/app.js")
	if asset.Code != http.StatusOK || asset.Body.String() != "primary asset" {
		t.Fatalf("admin GET /assets/app.js = %d %q, want primary asset", asset.Code, asset.Body.String())
	}

	home := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/")
	if home.Body.String() != "admin home" {
		t.Fatalf("admin GET / = %q, want admin home", home.Body.String())
	}

	missing := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/missing")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("admin GET /missing = %d, want 404", missing.Code)
	}
}

func TestIsolateReturnsNotFoundForPrimaryRoutes(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Get("/assets/app.js", textHandler("primary asset"))
	rt.Host(hostAdmin).Isolate()
	rt.Host(hostAdmin).Get("/{$}", textHandler("admin home"))

	asset := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/assets/app.js")
	if asset.Code != http.StatusNotFound {
		t.Fatalf("admin GET /assets/app.js = %d, want 404", asset.Code)
	}

	home := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/")
	if home.Code != http.StatusOK || home.Body.String() != "admin home" {
		t.Fatalf("admin GET / = %d %q, want 200 admin home", home.Code, home.Body.String())
	}
}

func TestHostMiddlewareOrder(t *testing.T) {
	t.Parallel()

	var order []string
	handler := func(name string) Handler {
		return func(ctx context.Context) error {
			order = append(order, name)
			return NoContent(ctx, http.StatusNoContent)
		}
	}

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Use(markMiddleware(&order, "global"))
	rt.Host(hostAdmin).Use(markMiddleware(&order, "host"))
	rt.Host(hostAdmin).FallbackToPrimary()
	rt.Host(hostAdmin).Get("/admin", handler("handler"), markMiddleware(&order, "route"))
	rt.Get("/assets/app.js", handler("asset"))

	serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/admin")
	if want := "global,host,route,handler"; strings.Join(order, ",") != want {
		t.Fatalf("order = %v, want %s", order, want)
	}

	order = order[:0]
	serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/assets/app.js")
	if want := "global,asset"; strings.Join(order, ",") != want {
		t.Fatalf("fallback order = %v, want %s (host middleware must not run)", order, want)
	}
}

func TestHostMiddlewareRunsForIsolatedNotFound(t *testing.T) {
	t.Parallel()

	var order []string
	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Host(hostAdmin).Use(markMiddleware(&order, "host"))
	rt.Host(hostAdmin).Isolate()
	rt.Host(hostAdmin).Get("/{$}", textHandler("admin"))

	recorder := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/missing")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if want := "host"; strings.Join(order, ",") != want {
		t.Fatalf("order = %v, want host", order)
	}
}

func TestHostMountServesEveryMethodUnderPrefix(t *testing.T) {
	t.Parallel()

	jobs := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Method + " " + r.URL.Path))
	})
	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Host(hostAdmin).Isolate()
	rt.Host(hostAdmin).Mount("/jobs", jobs)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/jobs"},
		{http.MethodPost, "/jobs/"},
		{http.MethodPut, "/jobs/42/retry"},
		{http.MethodPatch, "/jobs/42"},
		{http.MethodDelete, "/jobs/42"},
	}
	for _, tc := range cases {
		recorder := serveOnHost(rt, tc.method, "admin.andurel.com", tc.path)
		want := tc.method + " " + tc.path
		if recorder.Code != http.StatusOK || recorder.Body.String() != want {
			t.Fatalf("%s %s = %d %q, want 200 %q", tc.method, tc.path, recorder.Code, recorder.Body.String(), want)
		}
	}

	if got := serveOnHost(rt, http.MethodGet, "admin.andurel.com", "/jobsite"); got.Code != http.StatusNotFound {
		t.Fatalf("GET /jobsite = %d, want 404 (prefix is a path segment, not a string prefix)", got.Code)
	}
	if got := serveOnHost(rt, http.MethodGet, "andurel.com", "/jobs"); got.Code != http.StatusNotFound {
		t.Fatalf("primary GET /jobs = %d, want 404", got.Code)
	}
}

func TestHostSkipCSRFForHosts(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Use(CSRF(CSRFConfig{
		TokenLookup: "header:X-CSRF-Token",
		Skipper:     SkipCSRFForHosts(hostAdmin),
	}))
	rt.Post("/submit", func(ctx context.Context) error { return NoContent(ctx, http.StatusNoContent) })
	rt.Host(hostAdmin).Isolate()
	rt.Host(hostAdmin).Post("/submit", func(ctx context.Context) error {
		return NoContent(ctx, http.StatusNoContent)
	})

	admin := serveOnHost(rt, http.MethodPost, "admin.andurel.com", "/submit")
	if admin.Code != http.StatusNoContent {
		t.Fatalf("admin POST /submit without token = %d, want 204", admin.Code)
	}

	primary := serveOnHost(rt, http.MethodPost, "andurel.com", "/submit")
	if primary.Code != http.StatusForbidden {
		t.Fatalf("primary POST /submit without token = %d, want 403", primary.Code)
	}
}

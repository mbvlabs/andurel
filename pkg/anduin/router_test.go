package anduin

import (
	"context"
	"errors"
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

package anduin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
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

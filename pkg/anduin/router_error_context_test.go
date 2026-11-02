package anduin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// errorCtxKey is a middleware-provided context value. The router-level tests
// use it instead of a kiks type so pkg/anduin has no dependency on the session
// package. The published kiks/inertia reproduction is covered in T09.
type errorCtxKey struct{}

var errErrorCtxGuard = errors.New("anduin test: guard rejected")

// appendErrorCtx middleware records a value on the context it passes to next.
// A failing middleware placed after it therefore receives a context that is
// deeper than the one the router started with.
func appendErrorCtx(value string) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			current, _ := ctx.Value(errorCtxKey{}).(string)
			if current != "" {
				current += "+"
			}
			return next(context.WithValue(ctx, errorCtxKey{}, current+value))
		}
	}
}

// guardErrorCtx fails without calling next, like an authentication guard.
func guardErrorCtx(next Handler) Handler {
	return func(ctx context.Context) error {
		return Forbidden(errErrorCtxGuard)
	}
}

type errorRenderRecord struct {
	calls  int
	marker string
	err    error
	status int
}

func (record *errorRenderRecord) handler(body string) ErrorHandler {
	return func(ctx context.Context, err error) error {
		record.calls++
		record.marker, _ = ctx.Value(errorCtxKey{}).(string)
		record.err = err
		record.status = StatusCode(err)
		return Text(ctx, StatusCode(err), body)
	}
}

// errorCountingWriter counts every status and body write that reaches the
// actual response. Tests assert that an error page is written once.
type errorCountingWriter struct {
	header       http.Header
	status       int
	statusWrites int
	body         bytes.Buffer
	bodyWrites   int
}

func newErrorCountingWriter() *errorCountingWriter {
	return &errorCountingWriter{header: make(http.Header)}
}

func (writer *errorCountingWriter) Header() http.Header { return writer.header }

func (writer *errorCountingWriter) WriteHeader(status int) {
	writer.statusWrites++
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *errorCountingWriter) Write(body []byte) (int, error) {
	writer.bodyWrites++
	return writer.body.Write(body)
}

func serveErrorRequest(rt *Router, method, host, path string) *errorCountingWriter {
	request := httptest.NewRequest(method, path, nil)
	request.Host = host
	writer := newErrorCountingWriter()
	rt.Handler().ServeHTTP(writer, request)
	return writer
}

func newErrorContextRouter(t *testing.T, record *errorRenderRecord) *Router {
	t.Helper()
	return NewRouter(
		WithHosts(newAdminHosts(t)),
		WithErrorHandler(record.handler("themed")),
	)
}

func TestErrorHandlerReceivesDeepestMiddlewareContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		host       string
		path       string
		setup      func(t *testing.T, record *errorRenderRecord) *Router
		wantMarker string
		wantStatus int
		wantAllow  string
	}{
		{
			name:   "global guard",
			method: http.MethodGet,
			host:   "andurel.com",
			path:   "/",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"), guardErrorCtx)
				rt.Get("/", textHandler("home"))
				return rt
			},
			wantMarker: "G",
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "host guard",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/admin",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(guardErrorCtx)
				admin.Get("/admin", textHandler("admin"))
				return rt
			},
			wantMarker: "G",
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "host guard after host context middleware",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/admin",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(appendErrorCtx("H"), guardErrorCtx)
				admin.Get("/admin", textHandler("admin"))
				return rt
			},
			wantMarker: "G+H",
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "host context and handler error",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/admin",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(appendErrorCtx("H"))
				admin.Get("/admin", func(context.Context) error {
					return Forbidden(errErrorCtxGuard)
				})
				return rt
			},
			wantMarker: "G+H",
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "route context and handler error",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/admin",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(appendErrorCtx("H"))
				admin.Get("/admin", func(context.Context) error {
					return Forbidden(errErrorCtxGuard)
				}, appendErrorCtx("R"))
				return rt
			},
			wantMarker: "G+H+R",
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "route guard after route context middleware",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/admin",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(appendErrorCtx("H"))
				admin.Get("/admin", textHandler("never"), appendErrorCtx("R"), guardErrorCtx)
				return rt
			},
			wantMarker: "G+H+R",
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "isolated unknown path",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/missing",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(appendErrorCtx("H"))
				admin.Get("/admin", textHandler("admin"))
				return rt
			},
			wantMarker: "G+H",
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "isolated method mismatch",
			method: http.MethodPost,
			host:   "admin.andurel.com",
			path:   "/admin",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(appendErrorCtx("H"))
				admin.Get("/admin", textHandler("admin"))
				return rt
			},
			wantMarker: "G+H",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET, HEAD",
		},
		{
			name:   "fallback to primary handler error",
			method: http.MethodGet,
			host:   "admin.andurel.com",
			path:   "/asset",
			setup: func(t *testing.T, record *errorRenderRecord) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(appendErrorCtx("G"))
				rt.Get("/asset", func(context.Context) error {
					return Forbidden(errErrorCtxGuard)
				})
				admin := rt.Host(hostAdmin)
				admin.FallbackToPrimary()
				admin.Use(appendErrorCtx("H"))
				admin.Get("/admin", textHandler("admin"))
				return rt
			},
			wantMarker: "G",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := &errorRenderRecord{}
			rt := tc.setup(t, record)
			writer := serveErrorRequest(rt, tc.method, tc.host, tc.path)

			if record.calls != 1 {
				t.Fatalf("ErrorHandler calls = %d, want 1", record.calls)
			}
			if record.marker != tc.wantMarker {
				t.Fatalf("ErrorHandler context marker = %q, want %q", record.marker, tc.wantMarker)
			}
			if record.status != tc.wantStatus {
				t.Fatalf("ErrorHandler StatusCode = %d, want %d", record.status, tc.wantStatus)
			}
			if writer.status != tc.wantStatus {
				t.Fatalf("response status = %d, want %d", writer.status, tc.wantStatus)
			}
			if got := writer.header.Get("Allow"); got != tc.wantAllow {
				t.Fatalf("Allow = %q, want %q", got, tc.wantAllow)
			}
			if got, want := writer.body.String(), "themed"; got != want {
				t.Fatalf("body = %q, want %q", got, want)
			}
			if writer.statusWrites != 1 {
				t.Fatalf("underlying status writes = %d, want 1", writer.statusWrites)
			}
			if writer.bodyWrites != 1 {
				t.Fatalf("underlying body writes = %d, want 1", writer.bodyWrites)
			}
		})
	}
}

// TestErrorHandlerContextProvider exercises the B7-005 shape without importing
// kiks: the error page renderer fails unless the middleware-attached context is
// present. With the pre-middleware context the provider returns an error and
// the router falls back to a plain 500.
func TestErrorHandlerContextProvider(t *testing.T) {
	t.Parallel()

	record := &errorRenderRecord{}
	rt := NewRouter(
		WithHosts(newAdminHosts(t)),
		WithErrorHandler(func(ctx context.Context, err error) error {
			record.calls++
			record.marker, _ = ctx.Value(errorCtxKey{}).(string)
			if record.marker == "" {
				return errors.New("provider: middleware context missing")
			}
			return Text(ctx, StatusCode(err), "themed provider")
		}),
	)
	rt.Use(appendErrorCtx("G"), guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if record.calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", record.calls)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", writer.status)
	}
	if got, want := writer.body.String(), "themed provider"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

// TestErrorAfterResponseWriteStillRendersOnce pins response commitment: a
// handler that writes a response and then returns an error still reaches the
// ErrorHandler exactly once. The first write owns the status; the router must
// not add a second render of its own.
func TestErrorAfterResponseWriteStillRendersOnce(t *testing.T) {
	t.Parallel()

	record := &errorRenderRecord{}
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		record.calls++
		return Text(ctx, http.StatusInternalServerError, "error page")
	}))
	rt.Get("/", func(ctx context.Context) error {
		if err := Text(ctx, http.StatusOK, "partial"); err != nil {
			return err
		}
		return errors.New("after response")
	})

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if record.calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", record.calls)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("status = %d, want the first committed 200", writer.status)
	}
	if got, want := writer.body.String(), "partialerror page"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestDefaultErrorHandlerStaysPlain(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Use(appendErrorCtx("G"), guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", writer.status)
	}
	if got, want := writer.body.String(), "Forbidden\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if writer.statusWrites != 1 || writer.bodyWrites != 1 {
		t.Fatalf("writes = status %d body %d, want 1/1", writer.statusWrites, writer.bodyWrites)
	}
}

func TestErrorHandlerFailureFallsBackOnce(t *testing.T) {
	t.Parallel()

	renders := 0
	rt := NewRouter(WithErrorHandler(func(context.Context, error) error {
		renders++
		return errors.New("error renderer failed")
	}))
	rt.Use(appendErrorCtx("G"), guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", renders)
	}
	if writer.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", writer.status)
	}
	if got, want := writer.body.String(), "Internal Server Error\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if writer.statusWrites != 1 || writer.bodyWrites != 1 {
		t.Fatalf("writes = status %d body %d, want 1/1", writer.statusWrites, writer.bodyWrites)
	}
}

// TestRouteErrorObservationAndUnwindContract pins the existing contract:
// route middleware sees the error its handler returned, while host and global
// middleware see a consumed route error because the route boundary renders it.
// The error handler must see errors.Is/errors.As through wrapper errors.
func TestRouteErrorObservationAndUnwindContract(t *testing.T) {
	t.Parallel()

	var (
		events   []string
		observed []error
	)
	sentinel := errors.New("route sentinel")
	record := &errorRenderRecord{}
	rt := newErrorContextRouter(t, record)
	rt.Use(appendErrorCtx("G"))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			events = append(events, "global:in")
			err := next(ctx)
			events = append(events, "global:out")
			observed = append(observed, err)
			return err
		}
	})
	admin := rt.Host(hostAdmin)
	admin.Isolate()
	admin.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			events = append(events, "host:in")
			err := next(ctx)
			events = append(events, "host:out")
			observed = append(observed, err)
			return err
		}
	})
	admin.Get("/admin", func(context.Context) error {
		events = append(events, "handler")
		return Forbidden(sentinel)
	}, func(next Handler) Handler {
		return func(ctx context.Context) error {
			events = append(events, "route:in")
			err := next(ctx)
			events = append(events, "route:out")
			observed = append(observed, err)
			return fmt.Errorf("route wrapper: %w", err)
		}
	})

	writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
	if got, want := strings.Join(events, ","), "global:in,host:in,route:in,handler,route:out,host:out,global:out"; got != want {
		t.Fatalf("events = %s, want %s", got, want)
	}
	if len(observed) != 3 {
		t.Fatalf("observed errors = %d, want 3", len(observed))
	}
	if !errors.Is(observed[0], sentinel) {
		t.Fatalf("route middleware error = %v, want sentinel", observed[0])
	}
	if observed[1] != nil || observed[2] != nil {
		t.Fatalf("host/global observed route error = %v / %v, want nil (route boundary consumes it)", observed[1], observed[2])
	}
	if !errors.Is(record.err, sentinel) {
		t.Fatalf("ErrorHandler err = %v, want sentinel", record.err)
	}
	var httpErr *HTTPError
	if !errors.As(record.err, &httpErr) || httpErr.Status != http.StatusForbidden {
		t.Fatalf("ErrorHandler err = %v, want *HTTPError 403", record.err)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", writer.status)
	}
	if writer.statusWrites != 1 || writer.bodyWrites != 1 {
		t.Fatalf("writes = status %d body %d, want 1/1", writer.statusWrites, writer.bodyWrites)
	}
	if record.calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", record.calls)
	}
}

// TestHostAndGlobalErrorObservationAndUnwindContract pins that host and global
// errors still propagate through the middleware chain. Route errors are
// consumed at the route boundary; host and global middleware errors are not.
func TestHostAndGlobalErrorObservationAndUnwindContract(t *testing.T) {
	t.Parallel()

	t.Run("host error reaches global middleware", func(t *testing.T) {
		var (
			events   []string
			observed error
		)
		record := &errorRenderRecord{}
		rt := newErrorContextRouter(t, record)
		rt.Use(appendErrorCtx("G"))
		rt.Use(func(next Handler) Handler {
			return func(ctx context.Context) error {
				events = append(events, "global:in")
				err := next(ctx)
				events = append(events, "global:out")
				observed = err
				return err
			}
		})
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(appendErrorCtx("H"))
		admin.Use(func(next Handler) Handler {
			return func(ctx context.Context) error {
				events = append(events, "host:in")
				err := next(ctx)
				events = append(events, "host:out")
				return err
			}
		})
		admin.Use(guardErrorCtx)
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if got, want := strings.Join(events, ","), "global:in,host:in,host:out,global:out"; got != want {
			t.Fatalf("events = %s, want %s", got, want)
		}
		if !errors.Is(observed, errErrorCtxGuard) {
			t.Fatalf("global middleware observed %v, want guard error", observed)
		}
		if record.marker != "G+H" {
			t.Fatalf("ErrorHandler marker = %q, want G+H", record.marker)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})

	t.Run("global error reaches outer middleware", func(t *testing.T) {
		var observed error
		record := &errorRenderRecord{}
		rt := newErrorContextRouter(t, record)
		rt.Use(appendErrorCtx("G1"))
		rt.Use(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				observed = err
				return err
			}
		})
		rt.Use(appendErrorCtx("G2"), guardErrorCtx)
		rt.Get("/", textHandler("home"))

		writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
		if !errors.Is(observed, errErrorCtxGuard) {
			t.Fatalf("outer middleware observed %v, want guard error", observed)
		}
		if record.marker != "G1+G2" {
			t.Fatalf("ErrorHandler marker = %q, want G1+G2", record.marker)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})
}

// TestSwallowedMiddlewareErrorIsNotRendered pins that a global middleware that
// optionally delays errors with CatchErrors can turn an error into its own
// response. The scope prevents the inner boundary from rendering before the
// consumer runs; returning nil means the router never renders the error.
func TestSwallowedMiddlewareErrorIsNotRendered(t *testing.T) {
	t.Parallel()

	record := &errorRenderRecord{}
	rt := newErrorContextRouter(t, record)
	rt.Use(CatchErrors(func(next Handler) Handler {
		return func(ctx context.Context) error {
			if err := next(ctx); err != nil {
				return Text(ctx, http.StatusOK, "recovered by middleware")
			}
			return nil
		}
	}))
	rt.Use(appendErrorCtx("G"), guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if record.calls != 0 {
		t.Fatalf("ErrorHandler calls = %d, want 0 for a swallowed error", record.calls)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("status = %d, want 200", writer.status)
	}
	if got, want := writer.body.String(), "recovered by middleware"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

// errorPersistWriter mirrors kiks' persistWriter shape: it persists cookies on
// the first write and again (idempotently) when the middleware unwinds.
type errorPersistWriter struct {
	http.ResponseWriter
	status    int
	persisted bool
	saves     int
}

func (writer *errorPersistWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
	writer.persist()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *errorPersistWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	writer.persist()
	return writer.ResponseWriter.Write(body)
}

func (writer *errorPersistWriter) persist() {
	if writer.persisted {
		return
	}
	writer.persisted = true
	writer.saves++
	status := writer.status
	if status == 0 {
		status = http.StatusOK
	}
	http.SetCookie(writer.ResponseWriter, &http.Cookie{
		Name:  "session",
		Value: fmt.Sprintf("status=%d", status),
		Path:  "/",
	})
}

func errorPersistMiddleware(saves *int) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			writer := &errorPersistWriter{ResponseWriter: ResponseWriter(ctx)}
			ctx = ReplaceResponseWriter(ctx, writer)
			err := next(ctx)
			if !writer.persisted {
				writer.persist()
			}
			*saves = writer.saves
			return err
		}
	}
}

func TestErrorResponseCookieTiming(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(t *testing.T, record *errorRenderRecord, saves *int) *Router
		host  string
		path  string
	}{
		{
			name: "route handler error",
			setup: func(t *testing.T, record *errorRenderRecord, saves *int) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(errorPersistMiddleware(saves))
				rt.Use(appendErrorCtx("G"))
				rt.Get("/", func(ctx context.Context) error {
					return Forbidden(errErrorCtxGuard)
				})
				return rt
			},
			host: "andurel.com",
			path: "/",
		},
		{
			name: "host middleware error",
			setup: func(t *testing.T, record *errorRenderRecord, saves *int) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(errorPersistMiddleware(saves))
				rt.Use(appendErrorCtx("G"))
				admin := rt.Host(hostAdmin)
				admin.Isolate()
				admin.Use(guardErrorCtx)
				admin.Get("/admin", textHandler("admin"))
				return rt
			},
			host: "admin.andurel.com",
			path: "/admin",
		},
		{
			name: "global middleware error",
			setup: func(t *testing.T, record *errorRenderRecord, saves *int) *Router {
				rt := newErrorContextRouter(t, record)
				rt.Use(errorPersistMiddleware(saves))
				rt.Use(appendErrorCtx("G"), guardErrorCtx)
				rt.Get("/", textHandler("home"))
				return rt
			},
			host: "andurel.com",
			path: "/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := &errorRenderRecord{}
			cookieHandler := record.handler("themed")
			withCookie := func(ctx context.Context, err error) error {
				http.SetCookie(ResponseWriter(ctx), &http.Cookie{
					Name:  "rendered",
					Value: "yes",
					Path:  "/",
				})
				return cookieHandler(ctx, err)
			}
			saves := 0
			rt := tc.setup(t, record, &saves)
			rt.onError = withCookie

			writer := serveErrorRequest(rt, http.MethodGet, tc.host, tc.path)
			if record.calls != 1 {
				t.Fatalf("ErrorHandler calls = %d, want 1", record.calls)
			}
			if writer.status != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", writer.status)
			}
			if saves != 1 {
				t.Fatalf("cookie persist calls = %d, want 1", saves)
			}
			cookies := writer.header.Values("Set-Cookie")
			joined := strings.Join(cookies, "\n")
			if !strings.Contains(joined, "session=") {
				t.Fatalf("session cookie missing from %q", joined)
			}
			if !strings.Contains(joined, "rendered=yes") {
				t.Fatalf("error-render cookie missing from %q", joined)
			}
			if writer.statusWrites != 1 || writer.bodyWrites != 1 {
				t.Fatalf("writes = status %d body %d, want 1/1", writer.statusWrites, writer.bodyWrites)
			}
		})
	}
}

// TestErrorHandlerReceivesLiveContextAndActiveResources is the accepted
// contract for host and global errors: the error is rendered at the failing
// boundary while the middleware that created the child context and resource is
// still active, so cancellation and cleanup have not run yet.
func TestErrorHandlerReceivesLiveContextAndActiveResources(t *testing.T) {
	t.Parallel()

	type resourceKey struct{}
	type resourceState struct{ closed bool }

	setup := func(t *testing.T, host bool, events *[]string) (*Router, *errorRenderRecord, *resourceState) {
		t.Helper()

		resource := &resourceState{}
		record := &errorRenderRecord{}
		rt := newErrorContextRouter(t, record)
		rt.onError = func(ctx context.Context, err error) error {
			*events = append(*events, "render")
			if ctx.Err() != nil {
				t.Errorf("ErrorHandler ctx.Err = %v, want nil", ctx.Err())
			}
			if cause := context.Cause(ctx); cause != nil {
				t.Errorf("ErrorHandler context.Cause = %v, want nil", cause)
			}
			seen, _ := ctx.Value(resourceKey{}).(*resourceState)
			if seen != resource {
				t.Errorf("ErrorHandler resource = %p, want %p", seen, resource)
			}
			if seen != nil && seen.closed {
				t.Errorf("ErrorHandler resource already closed; want enclosing middleware resources active")
			}
			return record.handler("themed")(ctx, err)
		}

		withResource := func(next Handler) Handler {
			return func(ctx context.Context) error {
				child, cancel := context.WithCancel(ctx)
				defer func() {
					*events = append(*events, "cancel")
					cancel()
				}()
				defer func() {
					*events = append(*events, "cleanup")
					resource.closed = true
				}()
				*events = append(*events, "next")
				return next(context.WithValue(child, resourceKey{}, resource))
			}
		}

		if host {
			rt.Use(appendErrorCtx("G"))
			admin := rt.Host(hostAdmin)
			admin.Isolate()
			admin.Use(withResource, guardErrorCtx)
			admin.Get("/admin", textHandler("admin"))
			return rt, record, resource
		}

		rt.Use(withResource, guardErrorCtx)
		rt.Get("/", textHandler("home"))
		return rt, record, resource
	}

	t.Run("global", func(t *testing.T) {
		var events []string
		rt, record, _ := setup(t, false, &events)
		writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
		if got, want := strings.Join(events, ","), "next,render,cleanup,cancel"; got != want {
			t.Fatalf("events = %q, want %q", got, want)
		}
		if record.calls != 1 || writer.status != http.StatusForbidden {
			t.Fatalf("renders = %d, status = %d; want 1 and 403", record.calls, writer.status)
		}
		if !errors.Is(record.err, errErrorCtxGuard) {
			t.Fatalf("ErrorHandler err = %v, want the original guard error", record.err)
		}
		var httpErr *HTTPError
		if !errors.As(record.err, &httpErr) || httpErr.Status != http.StatusForbidden {
			t.Fatalf("ErrorHandler err = %v, want *HTTPError 403", record.err)
		}
	})

	t.Run("host", func(t *testing.T) {
		var events []string
		rt, record, _ := setup(t, true, &events)
		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if got, want := strings.Join(events, ","), "next,render,cleanup,cancel"; got != want {
			t.Fatalf("events = %q, want %q", got, want)
		}
		if record.calls != 1 || writer.status != http.StatusForbidden {
			t.Fatalf("renders = %d, status = %d; want 1 and 403", record.calls, writer.status)
		}
	})
}

// TestRouteScopeRetainsValuesButResourcesAreUnwound documents the deliberate
// route-scope limit: route errors render after the route middleware unwinds, so
// middleware values remain available but that middleware's own resources are
// already released. Cancellation still comes from the live request context.
func TestRouteScopeRetainsValuesButResourcesAreUnwound(t *testing.T) {
	t.Parallel()

	type resourceKey struct{}
	type resourceState struct{ closed bool }

	var (
		events   []string
		resource = &resourceState{}
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		events = append(events, "render")
		if ctx.Err() != nil {
			t.Errorf("ErrorHandler ctx.Err = %v, want live request context", ctx.Err())
		}
		marker, _ := ctx.Value(errorCtxKey{}).(string)
		if marker != "R" {
			t.Errorf("ErrorHandler marker = %q, want R", marker)
		}
		seen, _ := ctx.Value(resourceKey{}).(*resourceState)
		if seen != resource {
			t.Errorf("ErrorHandler resource = %p, want %p", seen, resource)
		}
		if seen != nil && !seen.closed {
			t.Errorf("route resource still open; route scope renders after unwind")
		}
		return Text(ctx, StatusCode(err), "themed")
	}))
	rt.Get("/", func(context.Context) error {
		return Forbidden(errErrorCtxGuard)
	}, func(next Handler) Handler {
		return func(ctx context.Context) error {
			child, cancel := context.WithCancel(ctx)
			defer func() {
				events = append(events, "cancel")
				cancel()
			}()
			defer func() {
				events = append(events, "cleanup")
				resource.closed = true
			}()
			events = append(events, "next")
			child = context.WithValue(child, errorCtxKey{}, "R")
			return next(context.WithValue(child, resourceKey{}, resource))
		}
	})

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if got, want := strings.Join(events, ","), "next,cleanup,cancel,render"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", writer.status)
	}
}

// errorBagKey fake-modelled kiks bag persistence. The save happens on the first
// response write, exactly like kiks' persistWriter, so early rendering must
// write through it before the bag middleware unwinds.
type errorBagKey struct{}

type errorBag struct {
	flash string
	dirty bool
}

func errorBagAddFlash(ctx context.Context, message string) {
	bag, _ := ctx.Value(errorBagKey{}).(*errorBag)
	if bag == nil {
		return
	}
	bag.flash = message
	bag.dirty = true
}

type errorBagResult struct {
	saveStatus int
	flashSaved bool
	events     []string
}

type errorBagWriter struct {
	http.ResponseWriter
	bag    *errorBag
	result *errorBagResult
	status int
	saved  bool
}

func (writer *errorBagWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
	writer.save()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *errorBagWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	writer.save()
	return writer.ResponseWriter.Write(body)
}

func (writer *errorBagWriter) save() {
	if writer.saved {
		return
	}
	writer.saved = true
	writer.result.events = append(writer.result.events, "save")

	status := writer.status
	if status == 0 {
		status = http.StatusOK
	}
	writer.result.saveStatus = status

	// Mirrors kiks shouldPersistFlashes: 3xx or 409.
	persistFlash := status >= 300 && status < 400 || status == http.StatusConflict
	if !persistFlash || writer.bag == nil || !writer.bag.dirty || writer.bag.flash == "" {
		return
	}
	writer.result.flashSaved = true
	http.SetCookie(writer.ResponseWriter, &http.Cookie{
		Name:  "flash",
		Value: writer.bag.flash,
		Path:  "/",
	})
}

func errorBagMiddleware(result *errorBagResult) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			bag := &errorBag{}
			writer := &errorBagWriter{ResponseWriter: ResponseWriter(ctx), bag: bag, result: result}
			ctx = context.WithValue(ctx, errorBagKey{}, bag)
			ctx = ReplaceResponseWriter(ctx, writer)

			err := next(ctx)
			if !writer.saved {
				writer.save()
			}
			return err
		}
	}
}

// TestErrorRenderSeesFinalBagStatus pins that bag persistence stays inside the
// bag middleware and observes the final status. A 409 error page written by the
// ErrorHandler is saved with its flash; a 303 redirect saves its status; a 403
// is still a clear (no flash) but the page write happens before cleanup.
func TestErrorRenderSeesFinalBagStatus(t *testing.T) {
	t.Parallel()

	t.Run("409 error page saves flash with final status", func(t *testing.T) {
		result := &errorBagResult{}
		rt := NewRouter(
			WithHosts(newAdminHosts(t)),
			WithErrorHandler(func(ctx context.Context, err error) error {
				result.events = append(result.events, "render")
				errorBagAddFlash(ctx, "conflict-flash")
				return Text(ctx, http.StatusConflict, "conflict page")
			}),
		)
		rt.Use(errorBagMiddleware(result))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(guardErrorCtx)
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		cookies := strings.Join(writer.header.Values("Set-Cookie"), "\n")
		if result.saveStatus != http.StatusConflict || !result.flashSaved ||
			!strings.Contains(cookies, "flash=conflict-flash") {
			t.Fatalf(
				"bag save status=%d flashSaved=%t cookies=%q; want 409 and the flashed cookie persisted",
				result.saveStatus,
				result.flashSaved,
				cookies,
			)
		}
		if got, want := strings.Join(result.events, ","), "render,save"; got != want {
			t.Fatalf("events = %q, want render before save (%q)", got, want)
		}
		if writer.status != http.StatusConflict || writer.body.String() != "conflict page" {
			t.Fatalf("response = %d %q, want 409 conflict page", writer.status, writer.body.String())
		}
	})

	t.Run("303 redirect saves final status", func(t *testing.T) {
		result := &errorBagResult{}
		rt := NewRouter(WithHosts(newAdminHosts(t)))
		rt.Use(errorBagMiddleware(result))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(func(next Handler) Handler {
			return func(ctx context.Context) error {
				return Redirect(ctx, "/sign-in", http.StatusSeeOther)
			}
		})
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/missing")
		if result.saveStatus != http.StatusSeeOther {
			t.Fatalf("bag save status = %d, want 303", result.saveStatus)
		}
		if writer.status != http.StatusSeeOther || writer.header.Get("Location") != "/sign-in" {
			t.Fatalf("response = %d Location %q, want 303 /sign-in", writer.status, writer.header.Get("Location"))
		}
	})

	t.Run("403 clear control", func(t *testing.T) {
		result := &errorBagResult{}
		rt := NewRouter(
			WithHosts(newAdminHosts(t)),
			WithErrorHandler(func(ctx context.Context, err error) error {
				result.events = append(result.events, "render")
				errorBagAddFlash(ctx, "must-not-persist")
				return Text(ctx, http.StatusForbidden, "forbidden page")
			}),
		)
		rt.Use(errorBagMiddleware(result))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(guardErrorCtx)
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if result.saveStatus != http.StatusForbidden || result.flashSaved {
			t.Fatalf("bag save status=%d flashSaved=%t; want 403 clear", result.saveStatus, result.flashSaved)
		}
		if got, want := strings.Join(result.events, ","), "render,save"; got != want {
			t.Fatalf("events = %q, want render before save (%q)", got, want)
		}
		if writer.status != http.StatusForbidden || writer.body.String() != "forbidden page" {
			t.Fatalf("response = %d %q, want 403 forbidden page", writer.status, writer.body.String())
		}
	})
}

// TestRouteScopeRetryUsesLatestFailingPass pins the implicit route scope:
// middleware may retry with different contexts, the latest failing pass supplies
// the deepest values, and a later success means no ErrorHandler call.
func TestRouteScopeRetryUsesLatestFailingPass(t *testing.T) {
	t.Parallel()

	type passKey struct{}

	t.Run("fail then fail renders latest", func(t *testing.T) {
		var (
			observedMarker string
			observedErr    string
			renders        int
		)
		rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
			renders++
			observedMarker, _ = ctx.Value(passKey{}).(string)
			observedErr = err.Error()
			return Text(ctx, StatusCode(err), "themed")
		}))
		rt.Get("/", textHandler("unused"),
			func(next Handler) Handler {
				return func(ctx context.Context) error {
					_ = next(context.WithValue(ctx, passKey{}, "A"))
					return next(context.WithValue(ctx, passKey{}, "B"))
				}
			},
			func(next Handler) Handler {
				return func(ctx context.Context) error {
					pass, _ := ctx.Value(passKey{}).(string)
					return Forbidden(fmt.Errorf("guard %s: %w", pass, errErrorCtxGuard))
				}
			},
		)

		writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
		if renders != 1 || observedMarker != "B" || !strings.Contains(observedErr, "guard B") {
			t.Fatalf(
				"ErrorHandler renders=%d marker=%q err=%q; want one render with context B and the last failing pass",
				renders,
				observedMarker,
				observedErr,
			)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})

	t.Run("fail then success renders nothing", func(t *testing.T) {
		renders := 0
		rt := NewRouter(WithErrorHandler(func(context.Context, error) error {
			renders++
			return nil
		}))
		rt.Get("/", textHandler("unused"),
			func(next Handler) Handler {
				return func(ctx context.Context) error {
					_ = next(context.WithValue(ctx, passKey{}, "A"))
					return next(context.WithValue(ctx, passKey{}, "B"))
				}
			},
			func(next Handler) Handler {
				return func(ctx context.Context) error {
					pass, _ := ctx.Value(passKey{}).(string)
					if pass == "A" {
						return Forbidden(errErrorCtxGuard)
					}
					return Text(ctx, http.StatusOK, "recovered "+pass)
				}
			},
		)

		writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
		if renders != 0 {
			t.Fatalf("ErrorHandler renders = %d, want 0 after a successful retry", renders)
		}
		if writer.status != http.StatusOK || writer.body.String() != "recovered B" {
			t.Fatalf("response = %d %q, want 200 recovered B", writer.status, writer.body.String())
		}
	})
}

// TestHostUnscopedRetryIsBlockedAfterEarlyRender pins the compatibility limit:
// a host/global middleware that retries after an early render gets
// ErrResponseCommitted from the second pass instead of a second ErrorHandler
// call. Use CatchErrors for retries that must stay unrendered.
func TestHostUnscopedRetryIsBlockedAfterEarlyRender(t *testing.T) {
	t.Parallel()

	type passKey struct{}
	var (
		secondErr      error
		renderedAfter  error
		renders        int
		observedMarker string
		guardCalls     int
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		observedMarker, _ = ctx.Value(passKey{}).(string)
		return Text(ctx, StatusCode(err), "themed")
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			_ = next(context.WithValue(ctx, passKey{}, "A"))
			renderedAfter = RenderedError(ctx)
			secondErr = next(context.WithValue(ctx, passKey{}, "B"))
			return secondErr
		}
	})
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			guardCalls++
			pass, _ := ctx.Value(passKey{}).(string)
			return Forbidden(fmt.Errorf("guard %s: %w", pass, errErrorCtxGuard))
		}
	})
	rt.Get("/", textHandler("unused"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 || observedMarker != "A" {
		t.Fatalf("ErrorHandler renders=%d marker=%q; want one early render for pass A", renders, observedMarker)
	}
	if guardCalls != 1 {
		t.Fatalf("guard calls = %d, want 1 (second next blocked)", guardCalls)
	}
	if !errors.Is(secondErr, ErrResponseCommitted) {
		t.Fatalf("second next error = %v, want ErrResponseCommitted", secondErr)
	}
	if !errors.Is(renderedAfter, errErrorCtxGuard) {
		t.Fatalf("RenderedError = %v, want the first guard error", renderedAfter)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 from the first render", writer.status)
	}
}

// TestHostMiddlewareRunsBeforeIsolatedNotFoundAndMethodNotAllowed documents
// B7-007: Isolate host middleware runs before the host's mux, including the
// mux's own 404 and 405 responses, while fallback requests bypass it.
func TestHostMiddlewareRunsBeforeIsolatedNotFoundAndMethodNotAllowed(t *testing.T) {
	t.Parallel()

	var order []string
	record := &errorRenderRecord{}
	rt := newErrorContextRouter(t, record)
	rt.Use(appendErrorCtx("G"))
	admin := rt.Host(hostAdmin)
	admin.Use(markMiddleware(&order, "host"), appendErrorCtx("H"))
	admin.Isolate()
	admin.Get("/admin", textHandler("admin"))

	cases := []struct {
		method string
		path   string
		status int
		allow  string
	}{
		{http.MethodGet, "/missing", http.StatusNotFound, ""},
		{http.MethodPost, "/admin", http.StatusMethodNotAllowed, "GET, HEAD"},
	}
	for _, tc := range cases {
		order = order[:0]
		writer := serveErrorRequest(rt, tc.method, "admin.andurel.com", tc.path)
		if got, want := strings.Join(order, ","), "host"; got != want {
			t.Fatalf("%s %s middleware order = %q, want %q", tc.method, tc.path, got, want)
		}
		if writer.status != tc.status {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, writer.status, tc.status)
		}
		if got := writer.header.Get("Allow"); got != tc.allow {
			t.Fatalf("%s %s Allow = %q, want %q", tc.method, tc.path, got, tc.allow)
		}
		if record.marker != "G+H" {
			t.Fatalf("%s %s ErrorHandler marker = %q, want G+H", tc.method, tc.path, record.marker)
		}
		record.calls = 0
	}

	// A fallback request must run the global chain but not the host chain.
	fallbackOrder := []string{}
	fallbackRecord := &errorRenderRecord{}
	fallback := newErrorContextRouter(t, fallbackRecord)
	fallback.Use(appendErrorCtx("G"))
	fallback.Get("/asset", textHandler("primary asset"))
	fallbackAdmin := fallback.Host(hostAdmin)
	fallbackAdmin.FallbackToPrimary()
	fallbackAdmin.Use(markMiddleware(&fallbackOrder, "host"), appendErrorCtx("H"))
	fallbackAdmin.Get("/admin", textHandler("admin"))

	fallbackWriter := serveErrorRequest(fallback, http.MethodGet, "admin.andurel.com", "/asset")
	if fallbackWriter.status != http.StatusOK || fallbackWriter.body.String() != "primary asset" {
		t.Fatalf("fallback response = %d %q, want 200 primary asset", fallbackWriter.status, fallbackWriter.body.String())
	}
	if len(fallbackOrder) != 0 {
		t.Fatalf("host middleware ran on a fallback request: %v", fallbackOrder)
	}
	if fallbackRecord.calls != 0 {
		t.Fatalf("ErrorHandler calls on fallback success = %d, want 0", fallbackRecord.calls)
	}
}

func TestHostMiddlewareRedirectPrecedesIsolatedNotFound(t *testing.T) {
	t.Parallel()

	rt := NewRouter(WithHosts(newAdminHosts(t)))
	admin := rt.Host(hostAdmin)
	admin.Isolate()
	admin.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			return Redirect(ctx, "/sign-in", http.StatusSeeOther)
		}
	})
	admin.Get("/admin", textHandler("admin"))

	writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/missing")
	if writer.status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", writer.status)
	}
	if got, want := writer.header.Get("Location"), "/sign-in"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

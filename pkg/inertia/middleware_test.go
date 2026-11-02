package inertia

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
)

const (
	captureTestAdminHost routing.HostName = "admin"
	captureTestVersion                    = "capture-test"
)

type captureTestContextKey struct{}

var (
	errCaptureTestGuard   = errors.New("inertia test: guard rejected")
	errCaptureTestAuth    = errors.New("inertia test: authentication required")
	errCaptureTestCommit  = errors.New("inertia test: commit write failed")
	errCaptureTestReflash = errors.New("inertia test: reflash failed")
)

func captureTestHosts(t *testing.T) *routing.Hosts {
	t.Helper()

	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "example.com", Protocol: "https"},
		captureTestAdminHost: {
			Hostname: "admin.example.com",
		},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}
	return hosts
}

func newCaptureTestRenderer(t *testing.T, options ...Option) *Renderer {
	t.Helper()

	options = append([]Option{WithRoot(testRoot(new(RootData))), WithVersion(captureTestVersion)}, options...)
	renderer, err := newTestRenderer(options...)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return renderer
}

func serveCaptureTestRequest(
	rt *anduin.Router,
	method string,
	host string,
	path string,
	inertiaVisit bool,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Host = host
	if inertiaVisit {
		request.Header.Set(HeaderInertia, "true")
		request.Header.Set(HeaderVersion, captureTestVersion)
	}
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, request)
	return recorder
}

func captureTestGuard(next anduin.Handler) anduin.Handler {
	return func(context.Context) error {
		return anduin.Forbidden(errCaptureTestGuard)
	}
}

func captureTestChallengeGuard(next anduin.Handler) anduin.Handler {
	return func(ctx context.Context) error {
		anduin.SetHeader(ctx, "WWW-Authenticate", `Basic realm="capture-test"`)
		return anduin.Unauthorized(errCaptureTestAuth)
	}
}

type captureTestCounter struct {
	hits atomic.Int64
}

func (counter *captureTestCounter) Increment(context.Context, string, time.Duration) (int64, error) {
	return counter.hits.Add(1), nil
}

func testThemedErrorHandler(
	renderer *Renderer,
	status int,
	calls *int,
	renderedErr *error,
) anduin.ErrorHandler {
	return func(ctx context.Context, err error) error {
		*calls++
		*renderedErr = err
		return renderer.Page(ctx, "Errors/NotFound", nil).Status(status).Render()
	}
}

func TestCaptureWriterDiscardBufferedResetsResponseAndPreservesMetadataHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Base", "keep")
	recorder.Header().Add("Vary", "Accept-Encoding")

	writer := newCaptureWriter(recorder)
	writer.Header().Set("X-Success", "stale")
	writer.Header().Add("Vary", "Origin")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.Header().Set("X-XSS-Protection", "0")
	writer.Header().Set("Allow", "GET, HEAD")
	writer.Header().Set("Retry-After", "60")
	writer.Header().Set("WWW-Authenticate", `Basic realm="capture-test"`)
	writer.Header().Set("Proxy-Authenticate", `Basic realm="proxy-test"`)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	if _, err := writer.Write([]byte("success")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	writer.DiscardBuffered()
	writer.DiscardBuffered()

	if writer.status != 0 {
		t.Fatalf("status = %d, want reset to 0", writer.status)
	}
	if writer.body.Len() != 0 {
		t.Fatalf("body = %q, want reset", writer.body.String())
	}
	header := recorder.Header()
	if got := header.Get("X-Success"); got != "" {
		t.Fatalf("X-Success = %q, want discarded", got)
	}
	if got := header.Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want discarded", got)
	}
	if got := header.Get("X-Base"); got != "keep" {
		t.Fatalf("X-Base = %q, want keep", got)
	}
	if got := header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := header.Get("X-XSS-Protection"); got != "0" {
		t.Fatalf("X-XSS-Protection = %q, want 0", got)
	}
	if got := header.Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
	if got := header.Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
	if got := header.Get("WWW-Authenticate"); got != `Basic realm="capture-test"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if got := header.Get("Proxy-Authenticate"); got != `Basic realm="proxy-test"` {
		t.Fatalf("Proxy-Authenticate = %q", got)
	}
	vary := strings.Join(header.Values("Vary"), ",")
	if !strings.Contains(vary, "Accept-Encoding") || !strings.Contains(vary, "Origin") {
		t.Fatalf("Vary = %q, want the base and metadata tokens", vary)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("delivered body = %q, want nothing before commit", recorder.Body.String())
	}

	writer.WriteHeader(http.StatusInternalServerError)
	if _, err := writer.Write([]byte("error")); err != nil {
		t.Fatalf("Write error page: %v", err)
	}
	if err := writer.commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("delivered status = %d, want 500", recorder.Code)
	}
	if got := recorder.Body.String(); got != "error" {
		t.Fatalf("delivered body = %q, want only the error page", got)
	}
}

func TestMiddlewareCommitsRenderedGuardError(t *testing.T) {
	for _, scope := range []string{"global", "host"} {
		for _, inertiaVisit := range []bool{false, true} {
			name := scope + "/document"
			if inertiaVisit {
				name = scope + "/inertia visit"
			}

			t.Run(name, func(t *testing.T) {
				var rootData RootData
				renderer, err := newTestRenderer(
					WithRoot(testRoot(&rootData)),
					WithVersion(captureTestVersion),
				)
				if err != nil {
					t.Fatalf("NewRenderer: %v", err)
				}

				calls := 0
				var renderedErr error
				var returnedErr error
				rt := anduin.NewRouter(
					anduin.WithHosts(captureTestHosts(t)),
					anduin.WithErrorHandler(testThemedErrorHandler(
						renderer,
						http.StatusForbidden,
						&calls,
						&renderedErr,
					)),
				)
				rt.Use(func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						returnedErr = next(ctx)
						return returnedErr
					}
				})
				rt.Use(renderer.Middleware())

				method, host, path := http.MethodGet, "example.com", "/"
				if scope == "global" {
					rt.Use(captureTestGuard)
					rt.Get(path, func(ctx context.Context) error {
						return anduin.Text(ctx, http.StatusOK, "home")
					})
				} else {
					host, path = "admin.example.com", "/admin"
					admin := rt.Host(captureTestAdminHost)
					admin.Isolate()
					admin.Use(captureTestGuard)
					admin.Get(path, func(ctx context.Context) error {
						return anduin.Text(ctx, http.StatusOK, "admin")
					})
				}

				recorder := serveCaptureTestRequest(rt, method, host, path, inertiaVisit)

				if calls != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", calls)
				}
				if !errors.Is(renderedErr, errCaptureTestGuard) {
					t.Fatalf("ErrorHandler err = %v, want the guard error", renderedErr)
				}
				if returnedErr != renderedErr {
					t.Fatalf("returned err = %v, want the unchanged original error", returnedErr)
				}
				if recorder.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
				}
				if vary := recorder.Header().Get("Vary"); !strings.Contains(vary, HeaderInertia) {
					t.Fatalf("Vary = %q, want %s", vary, HeaderInertia)
				}

				if inertiaVisit {
					if got := recorder.Header().Get(HeaderInertia); got != "true" {
						t.Fatalf("%s = %q, want true", HeaderInertia, got)
					}
					if !strings.Contains(recorder.Body.String(), `"component":"Errors/NotFound"`) {
						t.Fatalf("body = %q, want the themed error page", recorder.Body.String())
					}
					return
				}

				if rootData.Page.Component != "Errors/NotFound" {
					t.Fatalf("document component = %q, want Errors/NotFound", rootData.Page.Component)
				}
				if got := recorder.Header().Get(HeaderInertia); got != "" {
					t.Fatalf("%s = %q, want no JSON visit header on a document", HeaderInertia, got)
				}
			})
		}
	}
}

func TestMiddlewarePreservesAllowOnInertiaMethodNotAllowed(t *testing.T) {
	renderer := newCaptureTestRenderer(t)

	calls := 0
	var renderedErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		calls++
		renderedErr = err
		return renderer.Page(ctx, "Errors/MethodNotAllowed", nil).
			Status(http.StatusMethodNotAllowed).
			Render()
	}))
	rt.Use(renderer.Middleware())
	rt.Get("/only-get", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "get")
	})

	recorder := serveCaptureTestRequest(rt, http.MethodPost, "example.com", "/only-get", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if !errors.Is(renderedErr, anduin.ErrMethodNotAllowed) {
		t.Fatalf("ErrorHandler err = %v, want ErrMethodNotAllowed", renderedErr)
	}
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusMethodNotAllowed, recorder.Body.String())
	}
	if got := recorder.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"component":"Errors/MethodNotAllowed"`) {
		t.Fatalf("body = %q, want the themed method-not-allowed page", body)
	}
}

func TestMiddlewarePreservesRetryAfterFromRateLimit(t *testing.T) {
	renderer := newCaptureTestRenderer(t)

	calls := 0
	var renderedErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		calls++
		renderedErr = err
		return renderer.Page(ctx, "Errors/TooManyRequests", nil).
			Status(http.StatusTooManyRequests).
			Render()
	}))
	rt.Use(renderer.Middleware())
	rt.Post("/limited", func(ctx context.Context) error {
		return anduin.NoContent(ctx, http.StatusNoContent)
	}, anduin.RateLimit(&captureTestCounter{}, 1, time.Minute, nil))

	first := serveCaptureTestRequest(rt, http.MethodPost, "example.com", "/limited", true)
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusNoContent)
	}

	limited := serveCaptureTestRequest(rt, http.MethodPost, "example.com", "/limited", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if renderedErr == nil {
		t.Fatal("ErrorHandler did not receive the rate-limit error")
	}
	if got := anduin.StatusCode(renderedErr); got != http.StatusTooManyRequests {
		t.Fatalf("ErrorHandler status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d, want %d: %s", limited.Code, http.StatusTooManyRequests, limited.Body.String())
	}
	if got := limited.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
	if body := limited.Body.String(); !strings.Contains(body, `"component":"Errors/TooManyRequests"`) {
		t.Fatalf("body = %q, want the themed rate-limit page", body)
	}
}

func TestMiddlewarePreservesAuthenticationChallenge(t *testing.T) {
	for _, inertiaVisit := range []bool{false, true} {
		name := "document"
		if inertiaVisit {
			name = "inertia visit"
		}

		t.Run(name, func(t *testing.T) {
			renderer := newCaptureTestRenderer(t)

			calls := 0
			var renderedErr error
			rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
				calls++
				renderedErr = err
				return renderer.Page(ctx, "Errors/Unauthorized", nil).
					Status(http.StatusUnauthorized).
					Render()
			}))
			rt.Use(renderer.Middleware())
			rt.Use(captureTestChallengeGuard)
			rt.Get("/private", func(ctx context.Context) error {
				return anduin.Text(ctx, http.StatusOK, "private")
			})

			recorder := serveCaptureTestRequest(rt, http.MethodGet, "example.com", "/private", inertiaVisit)

			if calls != 1 {
				t.Fatalf("ErrorHandler calls = %d, want 1", calls)
			}
			if !errors.Is(renderedErr, errCaptureTestAuth) {
				t.Fatalf("ErrorHandler err = %v, want the auth error", renderedErr)
			}
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			if got := recorder.Header().Get("WWW-Authenticate"); got != `Basic realm="capture-test"` {
				t.Fatalf("WWW-Authenticate = %q", got)
			}
		})
	}
}

func TestMiddlewarePreservesDownstreamSecurityAndCORSHeaders(t *testing.T) {
	renderer := newCaptureTestRenderer(t)

	calls := 0
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		calls++
		return renderer.Page(ctx, "Errors/NotFound", nil).Status(http.StatusForbidden).Render()
	}))
	rt.Use(renderer.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			anduin.SetHeader(ctx, "Content-Security-Policy", "default-src 'self'")
			anduin.SetHeader(ctx, "X-Frame-Options", "DENY")
			anduin.SetHeader(ctx, "Access-Control-Allow-Origin", "https://client.example")
			anduin.SetHeader(ctx, "Access-Control-Allow-Credentials", "true")
			anduin.SetHeader(ctx, "Vary", "Origin")
			return next(ctx)
		}
	})
	rt.Use(captureTestGuard)
	rt.Get("/", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "home")
	})

	recorder := serveCaptureTestRequest(rt, http.MethodGet, "example.com", "/", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	for key, want := range map[string]string{
		"Content-Security-Policy":          "default-src 'self'",
		"X-Frame-Options":                  "DENY",
		"Access-Control-Allow-Origin":      "https://client.example",
		"Access-Control-Allow-Credentials": "true",
	} {
		if got := recorder.Header().Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	vary := strings.Join(recorder.Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Origin") {
		t.Fatalf("Vary = %q, want the CORS Origin token", vary)
	}
	if !strings.Contains(vary, HeaderInertia) {
		t.Fatalf("Vary = %q, want %s", vary, HeaderInertia)
	}
}

func TestMiddlewareErrorPageUsesMiddlewareContextProvider(t *testing.T) {
	renderer := newCaptureTestRenderer(t, WithSharedProvider(func(ctx context.Context) (Props, error) {
		marker, _ := ctx.Value(captureTestContextKey{}).(string)
		if marker == "" {
			return nil, errors.New("shared provider missing middleware context")
		}
		return Props{"marker": marker}, nil
	}))

	calls := 0
	var renderedErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(testThemedErrorHandler(
		renderer,
		http.StatusForbidden,
		&calls,
		&renderedErr,
	)))
	rt.Use(renderer.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			return next(context.WithValue(ctx, captureTestContextKey{}, "G"))
		}
	})
	rt.Use(captureTestGuard)
	rt.Get("/", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "home")
	})

	recorder := serveCaptureTestRequest(rt, http.MethodGet, "example.com", "/", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"marker":"G"`) {
		t.Fatalf("body = %q, want the middleware-provided shared prop", body)
	}
}

func TestMiddlewareNormalizesRenderedRedirectAndReflashes(t *testing.T) {
	reflashCalls := 0
	renderer := newCaptureTestRenderer(t, WithReflash(func(context.Context) error {
		reflashCalls++
		return nil
	}))

	calls := 0
	var renderedErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		calls++
		renderedErr = err
		return LocationRedirect(ctx, "/login#notice", http.StatusFound)
	}))
	rt.Use(renderer.Middleware())
	rt.Use(captureTestGuard)
	rt.Post("/login", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "login")
	})

	recorder := serveCaptureTestRequest(rt, http.MethodPost, "example.com", "/login", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if !errors.Is(renderedErr, errCaptureTestGuard) {
		t.Fatalf("ErrorHandler err = %v, want the guard error", renderedErr)
	}
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	if got := recorder.Header().Get(HeaderRedirect); got != "/login#notice" {
		t.Fatalf("%s = %q, want /login#notice", HeaderRedirect, got)
	}
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want no plain redirect", got)
	}
	if got := recorder.Header().Get(HeaderInertia); got != "" {
		t.Fatalf("%s = %q, want no JSON visit header on a location response", HeaderInertia, got)
	}
	if reflashCalls != 1 {
		t.Fatalf("reflash calls = %d, want 1", reflashCalls)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
	if vary := recorder.Header().Get("Vary"); !strings.Contains(vary, HeaderInertia) {
		t.Fatalf("Vary = %q, want %s", vary, HeaderInertia)
	}
}

func TestMiddlewareRenderedRedirectReflashFailureDeliversResponse(t *testing.T) {
	reflashCalls := 0
	renderer := newCaptureTestRenderer(t, WithReflash(func(context.Context) error {
		reflashCalls++
		return errCaptureTestReflash
	}))

	calls := 0
	var renderedErr error
	var observedErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		calls++
		renderedErr = err
		return LocationRedirect(ctx, "/login#notice", http.StatusFound)
	}))
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			observedErr = next(ctx)
			return observedErr
		}
	})
	rt.Use(renderer.Middleware())
	rt.Use(captureTestGuard)
	rt.Post("/login", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "login")
	})

	recorder := serveCaptureTestRequest(rt, http.MethodPost, "example.com", "/login", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if reflashCalls != 1 {
		t.Fatalf("reflash calls = %d, want 1", reflashCalls)
	}
	if !errors.Is(renderedErr, errCaptureTestGuard) {
		t.Fatalf("ErrorHandler err = %v, want the guard error", renderedErr)
	}
	if observedErr == nil {
		t.Fatal("observer saw a nil returned error")
	}
	if !errors.Is(observedErr, errCaptureTestGuard) {
		t.Fatalf("returned error = %v, want the original guard error", observedErr)
	}
	if !errors.Is(observedErr, errCaptureTestReflash) {
		t.Fatalf("returned error = %v, want the reflash error", observedErr)
	}
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	if got := recorder.Header().Get(HeaderRedirect); got != "/login#notice" {
		t.Fatalf("%s = %q, want /login#notice", HeaderRedirect, got)
	}
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want no plain redirect", got)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestMiddlewareDiscardsCaptureWhileCatchErrorsIsPending(t *testing.T) {
	renderer := newCaptureTestRenderer(t)

	calls := 0
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		calls++
		return anduin.Text(ctx, anduin.StatusCode(err), "pending-error")
	}))
	rt.Use(anduin.CatchErrors(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			return next(ctx)
		}
	}))
	rt.Use(renderer.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			if err := renderer.Page(ctx, "Success", nil).Render(); err != nil {
				return err
			}
			return anduin.Forbidden(errCaptureTestGuard)
		}
	})
	rt.Get("/", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "home")
	})

	recorder := serveCaptureTestRequest(rt, http.MethodGet, "example.com", "/", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if body := recorder.Body.String(); body != "pending-error" {
		t.Fatalf("body = %q, want only the pending CatchErrors error", body)
	}
}

func TestMiddlewareDiscardsBufferedSuccessBeforeHostError(t *testing.T) {
	var capturedRoot RootData
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&capturedRoot)),
		WithVersion(captureTestVersion),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	calls := 0
	var renderedErr error
	staleHeader := ""
	baseHeader := ""
	securityHeader := ""
	corsHeader := ""
	rt := anduin.NewRouter(
		anduin.WithHosts(captureTestHosts(t)),
		anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			calls++
			renderedErr = err
			header := anduin.ResponseWriter(ctx).Header()
			staleHeader = header.Get("X-Success")
			baseHeader = header.Get("X-Base")
			securityHeader = header.Get("X-Content-Type-Options")
			corsHeader = header.Get("Access-Control-Allow-Origin")
			return renderer.Page(ctx, "Errors/NotFound", nil).Status(http.StatusForbidden).Render()
		}),
	)
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			anduin.SetHeader(ctx, "X-Base", "keep")
			return next(ctx)
		}
	})
	rt.Use(renderer.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			anduin.SetHeader(ctx, "X-Content-Type-Options", "nosniff")
			anduin.SetHeader(ctx, "Access-Control-Allow-Origin", "https://client.example")
			return next(ctx)
		}
	})

	admin := rt.Host(captureTestAdminHost)
	admin.Isolate()
	admin.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			if err := next(ctx); err != nil {
				return err
			}
			return anduin.Forbidden(errCaptureTestGuard)
		}
	})
	admin.Get("/admin", func(ctx context.Context) error {
		anduin.SetHeader(ctx, "X-Success", "stale")
		return renderer.Page(ctx, "Success", nil).Render()
	})

	recorder := serveCaptureTestRequest(rt, http.MethodGet, "admin.example.com", "/admin", true)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if !errors.Is(renderedErr, errCaptureTestGuard) {
		t.Fatalf("ErrorHandler err = %v, want the host error", renderedErr)
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if staleHeader != "" {
		t.Fatalf("ErrorHandler saw discarded success header X-Success = %q", staleHeader)
	}
	if baseHeader != "keep" {
		t.Fatalf("ErrorHandler saw X-Base = %q, want keep", baseHeader)
	}
	if securityHeader != "nosniff" {
		t.Fatalf("ErrorHandler saw X-Content-Type-Options = %q, want nosniff", securityHeader)
	}
	if corsHeader != "https://client.example" {
		t.Fatalf("ErrorHandler saw Access-Control-Allow-Origin = %q", corsHeader)
	}
	if got := recorder.Header().Get("X-Success"); got != "" {
		t.Fatalf("response kept discarded success header X-Success = %q", got)
	}
	if got := recorder.Header().Get("X-Base"); got != "keep" {
		t.Fatalf("response X-Base = %q, want keep", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("response X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://client.example" {
		t.Fatalf("response Access-Control-Allow-Origin = %q", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"component":"Errors/NotFound"`) {
		t.Fatalf("body = %q, want the error page", body)
	}
	if strings.Contains(body, `"component":"Success"`) {
		t.Fatalf("body = %q, contains the discarded success page", body)
	}
}

type failingCaptureWriter struct {
	header http.Header
	status int
	err    error
}

func (writer *failingCaptureWriter) Header() http.Header { return writer.header }

func (writer *failingCaptureWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *failingCaptureWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func TestMiddlewareCommitErrorPreservesOriginalError(t *testing.T) {
	renderer := newCaptureTestRenderer(t)

	calls := 0
	var renderedErr error
	var observedErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(testThemedErrorHandler(
		renderer,
		http.StatusForbidden,
		&calls,
		&renderedErr,
	)))
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			observedErr = next(ctx)
			return observedErr
		}
	})
	rt.Use(renderer.Middleware())
	rt.Use(captureTestGuard)
	rt.Get("/", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "home")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = "example.com"
	request.Header.Set(HeaderInertia, "true")
	request.Header.Set(HeaderVersion, captureTestVersion)
	writer := &failingCaptureWriter{
		header: make(http.Header),
		err:    errCaptureTestCommit,
	}
	rt.Handler().ServeHTTP(writer, request)

	if calls != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", calls)
	}
	if !errors.Is(renderedErr, errCaptureTestGuard) {
		t.Fatalf("ErrorHandler err = %v, want the guard error", renderedErr)
	}
	if observedErr == nil {
		t.Fatal("observer saw a nil returned error")
	}
	if !errors.Is(observedErr, errCaptureTestGuard) {
		t.Fatalf("returned error = %v, want the original guard error", observedErr)
	}
	if !errors.Is(observedErr, errCaptureTestCommit) {
		t.Fatalf("returned error = %v, want the commit error", observedErr)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusForbidden)
	}
}

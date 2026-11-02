package runtimeintegration_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/kiks"
)

var (
	errAPIAuth     = errors.New("integration: API authentication required")
	errNestedGuard = errors.New("integration: nested guard rejected")
	errRetryGuard  = errors.New("integration: retry guard rejected once")
	errRouteOuter  = errors.New("integration: route error for enclosing scope")
)

// TestRouteAPIErrorConsumerReturnsOneJSONResponse mirrors an
// APIDocumentationErrors-style route middleware: it consumes the route error,
// writes one JSON 401/422 and returns nil. The router must not render the
// themed error page, and the Inertia capture must not add one.
func TestRouteAPIErrorConsumerReturnsOneJSONResponse(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity} {
		for _, visit := range []bool{false, true} {
			name := http.StatusText(status) + "/document"
			if visit {
				name = http.StatusText(status) + "/inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, rootData := newIntegrationRenderer(t)
				jar, _ := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "api-user",
					IsAuthenticated: true,
				}, false)

				renders := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusInternalServerError).
							Render()
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())

				consumerCalls := 0
				consumer := func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						err := next(ctx)
						if err == nil {
							return nil
						}
						if !errors.Is(err, errAPIAuth) {
							return err
						}
						consumerCalls++

						return anduin.JSON(ctx, status, map[string]string{
							"error": http.StatusText(status),
						})
					}
				}

				rt.Get("/api/docs", func(ctx context.Context) error {
					if status == http.StatusUnprocessableEntity {
						return anduin.Error(ctx, status, errAPIAuth)
					}

					return anduin.Unauthorized(errAPIAuth)
				}, consumer)

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    primaryAuthority,
					path:    "/api/docs",
					inertia: visit,
					accept:  "application/json",
					cookies: seedCookies,
				})

				if renders != 0 {
					t.Fatalf("ErrorHandler calls = %d, want 0 for a consumed API error", renders)
				}
				if consumerCalls != 1 {
					t.Fatalf("API consumer calls = %d, want 1", consumerCalls)
				}
				if writer.status != status {
					t.Fatalf("status = %d, want %d: %s", writer.status, status, writer.bodyString())
				}
				if writer.headerWrites != 1 || writer.bodyWrites != 1 {
					t.Fatalf(
						"delivery writes = headers %d body %d, want 1/1",
						writer.headerWrites,
						writer.bodyWrites,
					)
				}
				wantBody := `{"error":"` + http.StatusText(status) + `"}`
				if got := writer.bodyString(); got != wantBody {
					t.Fatalf("body = %q, want %q", got, wantBody)
				}
				if body := writer.bodyString(); strings.Contains(body, `"component"`) ||
					strings.Contains(body, "data-themed") {
					t.Fatalf("API response rendered a themed page: %s", body)
				}
				if rootData.Page.Component != "" {
					t.Fatalf("root document was rendered for an API response: %q", rootData.Page.Component)
				}
			})
		}
	}
}

// TestNestedCatchErrorsConsumeAndRetry covers the additive delayed scope through
// the real router, kiks bag and Inertia middleware:
//   - a host CatchErrors observer defers to an enclosing global consumer;
//   - a host CatchErrors scope retries once and lets the success response commit.
func TestNestedCatchErrorsConsumeAndRetry(t *testing.T) {
	t.Run("nested host observer defers to global consumer", func(t *testing.T) {
		for _, visit := range []bool{false, true} {
			name := "document"
			if visit {
				name = "inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, _ := newIntegrationRenderer(t)
				jar, store := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "nested-before",
					IsAuthenticated: true,
				}, false)
				store.reset()

				renders := 0
				hostObserved := 0
				consumerCalls := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusInternalServerError).
							Render()
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(anduin.CatchErrors(func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						err := next(ctx)
						if err == nil {
							return nil
						}
						if !errors.Is(err, errNestedGuard) {
							return err
						}
						consumerCalls++
						if setErr := kiks.Set(ctx, &sessionState{
							UserID:          "nested-after",
							IsAuthenticated: true,
						}); setErr != nil {
							return setErr
						}

						return anduin.JSON(ctx, http.StatusUnprocessableEntity, map[string]string{
							"error": http.StatusText(http.StatusUnprocessableEntity),
						})
					}
				}))
				rt.Use(renderer.Middleware())

				admin := rt.Host(adminHost)
				admin.Isolate()
				admin.Use(anduin.CatchErrors(func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						err := next(ctx)
						if err != nil {
							hostObserved++
						}

						return err
					}
				}))
				admin.Use(func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						if setErr := kiks.Set(ctx, &sessionState{
							UserID:          "nested-before",
							IsAuthenticated: true,
						}); setErr != nil {
							return setErr
						}

						return anduin.Error(ctx, http.StatusUnprocessableEntity, errNestedGuard)
					}
				})
				admin.Get("/nested", func(ctx context.Context) error {
					return anduin.Text(ctx, http.StatusOK, "unreachable")
				})

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    adminAuthority,
					path:    "/nested",
					inertia: visit,
					accept:  "application/json",
					cookies: seedCookies,
				})

				if renders != 0 {
					t.Fatalf("ErrorHandler calls = %d, want 0 for a consumed nested error", renders)
				}
				if hostObserved != 1 {
					t.Fatalf("host observer calls = %d, want 1", hostObserved)
				}
				if consumerCalls != 1 {
					t.Fatalf("global consumer calls = %d, want 1", consumerCalls)
				}
				if writer.status != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422: %s", writer.status, writer.bodyString())
				}
				if got := writer.bodyString(); got != `{"error":"Unprocessable Entity"}` {
					t.Fatalf("body = %q, want the consumer's JSON 422", got)
				}
				if saves := store.saveCount(); saves != 1 {
					t.Fatalf("session saves = %d, want the consumer's single 422 save", saves)
				}

				sessionCookie := cookieByName(writer.cookies(), sessionCookieName)
				if sessionCookie == nil || sessionCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the consumer's session saved", sessionCookieName, sessionCookie)
				}
				follow := readBag(t, jar, sessionCookie)
				if follow.err != nil {
					t.Fatalf("follow-up bag read: %v", follow.err)
				}
				if follow.session == nil || follow.session.UserID != "nested-after" {
					t.Fatalf("persisted session = %+v, want nested-after", follow.session)
				}
			})
		}
	})

	t.Run("host CatchErrors retries once then commits success", func(t *testing.T) {
		for _, visit := range []bool{false, true} {
			name := "document"
			if visit {
				name = "inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, _ := newIntegrationRenderer(t)
				jar, _ := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "retry-user",
					IsAuthenticated: true,
				}, false)

				renders := 0
				attempts := 0
				retries := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusInternalServerError).
							Render()
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())

				admin := rt.Host(adminHost)
				admin.Isolate()
				admin.Use(anduin.CatchErrors(func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						err := next(ctx)
						if err == nil {
							return nil
						}
						if !errors.Is(err, errRetryGuard) {
							return err
						}
						retries++

						return next(ctx)
					}
				}))
				admin.Use(func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						attempts++
						if attempts == 1 {
							return anduin.Forbidden(errRetryGuard)
						}

						return next(ctx)
					}
				})
				admin.Get("/retry", func(ctx context.Context) error {
					return renderer.Page(ctx, "Dashboard", nil).Render()
				})

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    adminAuthority,
					path:    "/retry",
					inertia: visit,
					cookies: seedCookies,
				})

				if renders != 0 {
					t.Fatalf("ErrorHandler calls = %d, want 0 after a successful retry", renders)
				}
				if attempts != 2 {
					t.Fatalf("guard attempts = %d, want first failure plus retry", attempts)
				}
				if retries != 1 {
					t.Fatalf("retries = %d, want 1", retries)
				}
				if writer.status != http.StatusOK {
					t.Fatalf("status = %d, want 200: %s", writer.status, writer.bodyString())
				}
				if writer.headerWrites != 1 || writer.bodyWrites != 1 {
					t.Fatalf(
						"delivery writes = headers %d body %d, want 1/1",
						writer.headerWrites,
						writer.bodyWrites,
					)
				}
				assertThemedComponent(t, writer.bodyString(), "Dashboard")
				if strings.Contains(writer.bodyString(), `"component":"Errors/NotFound"`) {
					t.Fatalf("retried response contains an error page: %s", writer.bodyString())
				}
			})
		}
	})
}

// TestRouteErrorDefersToEnclosingCatchErrors covers the route half of the nested
// delayed-scope handoff: ServeMux has no error return, so a route error must be
// carried out through request state to a host or global CatchErrors owner that
// can consume it before the router renders.
func TestRouteErrorDefersToEnclosingCatchErrors(t *testing.T) {
	for _, outer := range []string{"global", "host"} {
		for _, visit := range []bool{false, true} {
			name := outer + "/document"
			if visit {
				name = outer + "/inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, rootData := newIntegrationRenderer(t)
				jar, store := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "route-before",
					IsAuthenticated: true,
				}, false)
				store.reset()

				renders := 0
				consumerCalls := 0
				consumer := func(next anduin.Handler) anduin.Handler {
					return func(ctx context.Context) error {
						err := next(ctx)
						if err == nil {
							return nil
						}
						if !errors.Is(err, errRouteOuter) {
							return err
						}
						consumerCalls++
						if setErr := kiks.Set(ctx, &sessionState{
							UserID:          "route-after",
							IsAuthenticated: true,
						}); setErr != nil {
							return setErr
						}

						return anduin.JSON(ctx, http.StatusUnprocessableEntity, map[string]string{
							"error": http.StatusText(http.StatusUnprocessableEntity),
						})
					}
				}

				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusInternalServerError).
							Render()
					}),
				)
				rt.Use(jar.Middleware())

				path := "/route-error"
				host := primaryAuthority
				handler := func(ctx context.Context) error {
					return anduin.Error(ctx, http.StatusUnprocessableEntity, errRouteOuter)
				}
				if outer == "global" {
					rt.Use(anduin.CatchErrors(consumer))
					rt.Use(renderer.Middleware())
					rt.Get(path, handler)
				} else {
					rt.Use(renderer.Middleware())
					host = adminAuthority
					admin := rt.Host(adminHost)
					admin.Isolate()
					admin.Use(anduin.CatchErrors(consumer))
					admin.Get(path, handler)
				}

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    host,
					path:    path,
					inertia: visit,
					accept:  "application/json",
					cookies: seedCookies,
				})

				if renders != 0 {
					t.Fatalf("ErrorHandler calls = %d, want 0 for a consumed route error", renders)
				}
				if consumerCalls != 1 {
					t.Fatalf("enclosing CatchErrors consumer calls = %d, want 1", consumerCalls)
				}
				if rootData.Page.Component != "" {
					t.Fatalf("themed root rendered for a consumed route error: %q", rootData.Page.Component)
				}
				if writer.status != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422: %s", writer.status, writer.bodyString())
				}
				if got := writer.bodyString(); got != `{"error":"Unprocessable Entity"}` {
					t.Fatalf("body = %q, want the consumer's JSON 422", got)
				}
				if writer.headerWrites != 1 || writer.bodyWrites != 1 {
					t.Fatalf(
						"delivery writes = headers %d body %d, want 1/1",
						writer.headerWrites,
						writer.bodyWrites,
					)
				}
				if saves := store.saveCount(); saves != 1 {
					t.Fatalf("session saves = %d, want the consumer's single 422 save", saves)
				}

				sessionCookie := cookieByName(writer.cookies(), sessionCookieName)
				if sessionCookie == nil || sessionCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the consumer's session saved", sessionCookieName, sessionCookie)
				}
				follow := readBag(t, jar, sessionCookie)
				if follow.err != nil {
					t.Fatalf("follow-up bag read: %v", follow.err)
				}
				if follow.session == nil || follow.session.UserID != "route-after" {
					t.Fatalf("persisted session = %+v, want route-after", follow.session)
				}
			})
		}
	}
}

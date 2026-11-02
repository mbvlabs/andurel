package runtimeintegration_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

var errResourceCanceled = errors.New("integration: resource scope cleaned up")

var errClientGone = errors.New("integration: synthetic client disconnected")

type resourceKey struct{}

// resourceObservation records whether an enclosing middleware's resource and
// cancellation context were still live when the early ErrorHandler ran, and
// whether cleanup happened only afterwards.
type resourceObservation struct {
	mu sync.Mutex

	active    bool
	enters    int
	cleanups  int
	ctx       context.Context
	handlerIn int

	handlerOpen  bool
	handlerValue bool
	handlerErr   error
	handlerCause error
}

func (observation *resourceObservation) entered(ctx context.Context) {
	observation.mu.Lock()
	defer observation.mu.Unlock()

	observation.enters++
	observation.active = true
	observation.ctx = ctx
}

func (observation *resourceObservation) cleaned() {
	observation.mu.Lock()
	defer observation.mu.Unlock()

	observation.cleanups++
	observation.active = false
}

func (observation *resourceObservation) observed(ctx context.Context) {
	observation.mu.Lock()
	defer observation.mu.Unlock()

	observation.handlerIn++
	observation.handlerOpen = observation.active
	_, observation.handlerValue = ctx.Value(resourceKey{}).(*resourceObservation)
	observation.handlerErr = ctx.Err()
	observation.handlerCause = context.Cause(ctx)
}

func (observation *resourceObservation) snapshot() (enters, cleanups, handlerIn int, active bool, ctx context.Context) {
	observation.mu.Lock()
	defer observation.mu.Unlock()

	return observation.enters, observation.cleanups, observation.handlerIn, observation.active, observation.ctx
}

func (observation *resourceObservation) handler() (open, value bool, err, cause error) {
	observation.mu.Lock()
	defer observation.mu.Unlock()

	return observation.handlerOpen, observation.handlerValue, observation.handlerErr, observation.handlerCause
}

// resourceMiddleware owns a cancelable child context and a synthetic resource.
// A guard deeper in the chain fails; the early host/global render must see the
// resource active and the context live, and cleanup must run only after the
// ErrorHandler has returned.
func resourceMiddleware(observation *resourceObservation) anduin.Middleware {
	return func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			child, cancel := context.WithCancelCause(ctx)
			observation.entered(child)
			defer func() {
				cancel(errResourceCanceled)
				observation.cleaned()
			}()

			return next(context.WithValue(child, resourceKey{}, observation))
		}
	}
}

func TestEnclosingMiddlewareResourcesAreLiveDuringErrorHandler(t *testing.T) {
	for _, scope := range []string{"global", "host"} {
		for _, visit := range []bool{false, true} {
			name := scope + "/document"
			if visit {
				name = scope + "/inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, _ := newIntegrationRenderer(t)
				jar, _ := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "resource-user",
					IsAuthenticated: true,
				}, false)

				observation := &resourceObservation{}
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						observation.observed(ctx)

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusForbidden).
							Render()
					}),
				)
				rt.Use(jar.Middleware())

				guard := func(next anduin.Handler) anduin.Handler {
					return func(context.Context) error {
						return anduin.Forbidden(errHostGuard)
					}
				}
				path := "/guarded"
				host := primaryAuthority
				if scope == "global" {
					rt.Use(resourceMiddleware(observation))
					rt.Use(renderer.Middleware())
					rt.Use(guard)
					rt.Get(path, func(ctx context.Context) error {
						return anduin.Text(ctx, http.StatusOK, "unreachable")
					})
				} else {
					rt.Use(renderer.Middleware())
					host = adminAuthority
					admin := rt.Host(adminHost)
					admin.Isolate()
					admin.Use(resourceMiddleware(observation))
					admin.Use(guard)
					admin.Get(path, func(ctx context.Context) error {
						return anduin.Text(ctx, http.StatusOK, "unreachable")
					})
				}

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    host,
					path:    path,
					inertia: visit,
					cookies: seedCookies,
				})

				enters, cleanups, handlerIn, active, resourceCtx := observation.snapshot()
				if enters != 1 {
					t.Fatalf("resource enters = %d, want 1", enters)
				}
				if handlerIn != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", handlerIn)
				}
				if cleanups != 1 {
					t.Fatalf("resource cleanups = %d, want 1", cleanups)
				}
				if active {
					t.Fatal("resource is still active after ServeHTTP")
				}

				open, value, handlerErr, handlerCause := observation.handler()
				if !open {
					t.Fatal("ErrorHandler ran after the resource was cleaned up")
				}
				if !value {
					t.Fatal("ErrorHandler did not receive the middleware resource value")
				}
				if handlerErr != nil {
					t.Fatalf("ErrorHandler ctx.Err() = %v, want a live context", handlerErr)
				}
				if handlerCause != nil {
					t.Fatalf("ErrorHandler context.Cause = %v, want nil during render", handlerCause)
				}

				if resourceCtx == nil {
					t.Fatal("resource middleware did not expose its child context")
				}
				if !errors.Is(context.Cause(resourceCtx), errResourceCanceled) {
					t.Fatalf("cleanup cause = %v, want errResourceCanceled", context.Cause(resourceCtx))
				}
				if !errors.Is(resourceCtx.Err(), context.Canceled) {
					t.Fatalf("cleaned context err = %v, want context.Canceled", resourceCtx.Err())
				}

				if writer.status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403: %s", writer.status, writer.bodyString())
				}
				assertThemedComponent(t, writer.bodyString(), "Errors/NotFound")
			})
		}
	}
}

// TestDelayedRouteErrorUsesLiveRequestCause protects the merged-context
// cancellation semantics: a route-scope render records values from the failing
// pass but must take Err and Cause from the live request, even when the request
// context was canceled with a custom cause before dispatch.
func TestDelayedRouteErrorUsesLiveRequestCause(t *testing.T) {
	for _, visit := range []bool{false, true} {
		name := "document"
		if visit {
			name = "inertia"
		}

		t.Run(name, func(t *testing.T) {
			renderer, _ := newIntegrationRenderer(t)
			jar, _ := newIntegrationJar(t)
			seedCookies := seedSession(t, jar, &sessionState{
				UserID:          "canceled-user",
				IsAuthenticated: true,
			}, false)

			var (
				renders      int
				handlerErr   error
				handlerCause error
			)
			rt := anduin.NewRouter(
				anduin.WithHosts(newIntegrationHosts(t)),
				anduin.WithErrorHandler(func(ctx context.Context, err error) error {
					renders++
					handlerErr = ctx.Err()
					handlerCause = context.Cause(ctx)

					return renderer.Page(ctx, "Errors/NotFound", nil).
						Status(http.StatusForbidden).
						Render()
				}),
			)
			rt.Use(jar.Middleware())
			rt.Use(renderer.Middleware())
			rt.Get("/canceled", func(context.Context) error {
				return anduin.Forbidden(errHostGuard)
			})

			requestCtx, cancel := context.WithCancelCause(context.Background())
			cancel(errClientGone)

			writer := serveRequest(t, rt.Handler(), requestOptions{
				method:  http.MethodGet,
				host:    primaryAuthority,
				path:    "/canceled",
				inertia: visit,
				ctx:     requestCtx,
				cookies: seedCookies,
			})

			if renders != 1 {
				t.Fatalf("ErrorHandler calls = %d, want 1", renders)
			}
			if !errors.Is(handlerErr, context.Canceled) {
				t.Fatalf("ErrorHandler ctx.Err() = %v, want context.Canceled", handlerErr)
			}
			if !errors.Is(handlerCause, errClientGone) {
				t.Fatalf("ErrorHandler context.Cause = %v, want the live custom cause", handlerCause)
			}
			if writer.status != http.StatusForbidden {
				t.Fatalf("status = %d, want 403: %s", writer.status, writer.bodyString())
			}
			assertThemedComponent(t, writer.bodyString(), "Errors/NotFound")
		})
	}
}

package runtimeintegration_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

var errHostGuard = errors.New("integration: host/global guard rejected")

// TestHostAndGlobalErrorsKeepKiksContext is the combined document/X-Inertia
// regression for the beta.7 B7-005 failure: a host or global middleware error
// must reach the ErrorHandler with the live kiks bag. The shared provider is
// strict: if kiks.Get returns ErrNoBag the themed page cannot render.
func TestHostAndGlobalErrorsKeepKiksContext(t *testing.T) {
	for _, scope := range []string{"global", "host"} {
		for _, visit := range []bool{false, true} {
			name := scope + "/document"
			if visit {
				name = scope + "/inertia"
			}

			t.Run(name, func(t *testing.T) {
				provider := &strictAuthProvider{}
				renderer, rootData := newIntegrationRenderer(
					t,
					inertia.WithSharedProvider(provider.provide),
				)
				jar, store := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "user-1",
					IsAuthenticated: true,
				}, false)
				store.reset()

				observation := &errorObservation{}
				themed := inertia.NewErrorHandler(
					renderer,
					inertia.ErrorHandlerOptions{},
					inertia.MapError(errHostGuard, http.StatusForbidden, "Errors/NotFound"),
				)
				onError := func(ctx context.Context, err error) error {
					observation.record(ctx, err)

					return themed(ctx, err)
				}

				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(onError),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())

				path := "/guarded"
				host := primaryAuthority
				if scope == "global" {
					rt.Use(func(next anduin.Handler) anduin.Handler {
						return func(context.Context) error {
							return anduin.Forbidden(errHostGuard)
						}
					})
					rt.Get(path, func(ctx context.Context) error {
						return anduin.Text(ctx, http.StatusOK, "unreachable")
					})
				} else {
					host = adminAuthority
					admin := rt.Host(adminHost)
					admin.Isolate()
					admin.Use(func(next anduin.Handler) anduin.Handler {
						return func(context.Context) error {
							return anduin.Forbidden(errHostGuard)
						}
					})
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

				if observation.calls != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", observation.calls)
				}
				if !errors.Is(observation.err, errHostGuard) {
					t.Fatalf("ErrorHandler err = %v, want the guard error", observation.err)
				}
				if observation.ctxErr != nil {
					t.Fatalf("ErrorHandler ctx.Err() = %v, want a live request context", observation.ctxErr)
				}
				if observation.getErr != nil {
					t.Fatalf("ErrorHandler kiks.Get = %v, want the request bag", observation.getErr)
				}
				assertAuthenticatedSession(t, observation.session)

				calls, providerErr, providerSession := provider.snapshot()
				if calls != 1 {
					t.Fatalf("strict provider calls = %d, want 1", calls)
				}
				if providerErr != nil {
					t.Fatalf("strict provider kiks.Get = %v, want no ErrNoBag", providerErr)
				}
				assertAuthenticatedSession(t, providerSession)

				if writer.status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403: %s", writer.status, writer.bodyString())
				}
				if vary := writer.header.Get("Vary"); !strings.Contains(vary, inertia.HeaderInertia) {
					t.Fatalf("Vary = %q, want %s", vary, inertia.HeaderInertia)
				}
				if strings.Contains(writer.bodyString(), "unreachable") {
					t.Fatalf("body leaked the unreachable handler: %s", writer.bodyString())
				}

				if !visit {
					if rootData.Page.Component != "Errors/NotFound" {
						t.Fatalf("document component = %q, want Errors/NotFound", rootData.Page.Component)
					}
					assertAuthProp(t, rootData.Page.Props)
					assertThemedComponent(t, writer.bodyString(), "Errors/NotFound")
					if !strings.Contains(writer.bodyString(), `data-themed="true"`) {
						t.Fatalf("document is not the themed root: %s", writer.bodyString())
					}

					return
				}

				if got := writer.header.Get(inertia.HeaderInertia); got != "true" {
					t.Fatalf("%s = %q, want true", inertia.HeaderInertia, got)
				}
				if contentType := writer.header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
					t.Fatalf("Content-Type = %q, want JSON", contentType)
				}
				page := assertInertiaJSONPage(t, writer.body.Bytes())
				if page.Component != "Errors/NotFound" {
					t.Fatalf("X-Inertia component = %q, want Errors/NotFound", page.Component)
				}
				assertAuthProp(t, page.Props)
			})
		}
	}
}

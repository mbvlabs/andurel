package runtimeintegration_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// TestInertiaCaptureDeliversRenderedErrorExactlyOnce pins the coordinated
// capture contract at the real writer: the error page reaches the HTTP sink with
// one status write and one body write, whether the request is a full document
// or an X-Inertia visit, and whether the guard is global or host-scoped.
func TestInertiaCaptureDeliversRenderedErrorExactlyOnce(t *testing.T) {
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
					UserID:          "capture-user",
					IsAuthenticated: true,
				}, false)

				renders := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusForbidden).
							Render()
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())

				guard := func(next anduin.Handler) anduin.Handler {
					return func(context.Context) error {
						return anduin.Forbidden(errHostGuard)
					}
				}
				path := "/capture"
				host := primaryAuthority
				if scope == "global" {
					rt.Use(guard)
					rt.Get(path, func(ctx context.Context) error {
						return anduin.Text(ctx, http.StatusOK, "unreachable")
					})
				} else {
					host = adminAuthority
					admin := rt.Host(adminHost)
					admin.Isolate()
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

				if renders != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", renders)
				}
				if writer.status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403: %s", writer.status, writer.bodyString())
				}
				if writer.headerWrites != 1 {
					t.Fatalf("header writes = %d, want exactly 1", writer.headerWrites)
				}
				if writer.bodyWrites != 1 {
					t.Fatalf("body writes = %d, want exactly 1", writer.bodyWrites)
				}
				if count := strings.Count(writer.bodyString(), `"component":"Errors/NotFound"`); count != 1 {
					t.Fatalf("error page component count = %d, want 1: %s", count, writer.bodyString())
				}
				if strings.Contains(writer.bodyString(), `"component":"Success"`) {
					t.Fatalf("body contains a success page: %s", writer.bodyString())
				}
			})
		}
	}
}

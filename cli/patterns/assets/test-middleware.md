# Test middleware

Middleware is ordinary `anduin.Handler` wrapping, so it can be tested without a
database or a full application. Build a real `httptest` server with
`pkg/anduin/anduintest` and register the middleware on a small route; add a
test-only middleware that attaches the identity or other context value the
middleware reads.

## Files

### middleware/require_admin_test.go
```go
package middleware_test

import (
	"context"
	"net/http"
	"testing"

	"app/domains"
	"app/middleware"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/anduin/anduintest"
)

func TestRequireAdminAllowsAdmin(t *testing.T) {
	srv := newAdminServer(t, true)

	resp := srv.Get("/admin")
	resp.AssertStatus(http.StatusOK)
}

func TestRequireAdminRejectsNonAdmin(t *testing.T) {
	srv := newAdminServer(t, false)

	resp := srv.Get("/admin")
	resp.AssertStatus(http.StatusForbidden)
}

func TestRequireAdminRedirectsWithoutViewer(t *testing.T) {
	srv := anduintest.New(t, func(rt *anduin.Router) {
		rt.Get("/admin", adminPage, middleware.RequireAdmin())
	})

	resp := srv.Get("/admin")
	resp.AssertStatus(http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != routes.SessionNew.URL() {
		t.Fatalf("location = %q, want %q", got, routes.SessionNew.URL())
	}
}

func newAdminServer(t *testing.T, admin bool) *anduintest.Server {
	t.Helper()

	viewer := func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			return next(middleware.WithViewer(ctx, domains.User{IsAdmin: admin}))
		}
	}

	return anduintest.New(t, func(rt *anduin.Router) {
		rt.Get("/admin", adminPage, viewer, middleware.RequireAdmin())
	})
}

func adminPage(ctx context.Context) error {
	return anduin.Text(ctx, http.StatusOK, "admin")
}
```

## Rules

- Test through `anduintest.New(t, register)` so the middleware, router, and
  cookie jar behave as they do in production; use `anduin.Serve` directly only
  when the middleware is a pure function of the context and needs no cookie or
  router behavior.
- Attach request-scoped state with a test-only middleware that calls the
  package's own `WithX(ctx, ...)` helper. Do not reach into unexported context
  keys or copy the middleware's internals.
- List test middleware before the middleware under test; the first middleware
  is the outermost and must run first to populate the context.
- Assert status codes and `Location` headers. For an error that the router
  should map, either install a `WithErrorHandler` or assert the status the
  default handler writes.
- Avoid a database unless the middleware itself reads one. `AuthOnly` tests
  need a real receiver and a seeded user; a policy guard that reads
  `ViewerFrom(ctx)` does not.
- Name the test package `middleware_test` and keep the helper functions in the
  test file. The full-scaffold compile test compiles every pattern test file.
- Add a case for each branch: allowed, rejected, and missing context value.

## Checklist

- [ ] Server built with `anduintest.New` from real registrations.
- [ ] Context state attached with the package's exported helper.
- [ ] Allowed, rejected, and missing-state branches covered.
- [ ] No database unless the middleware reads one.
- [ ] `go test ./middleware/...` passes.

# Middleware

Application HTTP policy lives in the single flat `middleware` package. A
middleware has the `anduin.Middleware` shape — `type Middleware func(Handler)
Handler` — and wraps an `anduin.Handler`.

Generic cross-cutting behavior (recovery, client IP, CORS, CSRF, rate limits,
telemetry) comes from `pkg/anduin`; `middleware/` is for policy this application
owns, such as an admin guard or a session definition. Install global middleware
with `router.Use(...)` in `app`, before `router.Handler()` is
called. Per-domain middleware goes on the host with `rt.Host(name).Use(...)`,
and per-route middleware goes after the handler:
`rt.Route(routes.AdminDashboard).Get(h, middleware.RequireAdmin())`.

## Files

### middleware/require_admin.go
```go
package middleware

import (
	"context"
	"errors"
	"net/http"

	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// RequireAdmin allows only a visitor AuthOnly loaded from the database and
// marked as an admin. Register it after AuthOnly so the viewer is present.
func RequireAdmin() anduin.Middleware {
	return func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			viewer, ok := ViewerFrom(ctx)
			if !ok {
				return anduin.Redirect(ctx, routes.SessionNew.URL(), http.StatusSeeOther)
			}

			if !viewer.IsAdmin {
				return anduin.Forbidden(errors.New("admin access required"))
			}

			return next(ctx)
		}
	}
}
```

## Rules

- One flat `package middleware`; no sub-packages and no import aliases.
- App policy goes here; generic middleware belongs to `pkg/anduin` and is used
  directly. Do not wrap or reimplement `Recover`, `RealIP`, `Logger`,
  `Telemetry`, `CORS`, or `CSRF`.
- Host-scoped policy branches on `anduin.RequestHost(ctx)` (the configured
  `routing.HostName` the router dispatches to); never re-parse
  `anduin.Request(ctx).Host`. The scaffold's unwired
  `middleware.DenyFraming(config.HostAdmin)` is the worked example.
- Register global middleware with `router.Use(...)` in `app`.
  `Use` must run before the first `router.Handler()` call, because the handler
  chain is built and cached once.
- Per-route middleware goes after the handler:
  `rt.Route(routes.AdminDashboard).Get(h.Dashboard, middleware.RequireAdmin())`.
  The first middleware in the list is the outermost, so list authentication
  before authorization.
- Per-domain middleware goes on the host, not in a path check:
  `admin := rt.Host(config.HostAdmin)`, `admin.FallbackToPrimary()` or
  `admin.Isolate()`, then `admin.Use(middleware.AuthOnly(users, hosts))`. It
  runs after global middleware and before per-route middleware, and only for
  requests the domain answers from its own routes. A request that falls back to
  the primary site's routes (an asset, say) does not run it.
- Never trust identity from a cookie. `AuthOnly(users, hosts)` reloads the user from
  the database and attaches it with `WithViewer`; read it with `ViewerFrom`.
  A deleted user or a stale `SessionVersion` is rejected on every request. A
  rejected visitor is sent to the primary host's sign-in page.
- Put request-scoped values on `context.Context` with unexported key types in
  the same package, and expose typed accessors (`WithViewer` / `ViewerFrom`).
  Dependencies and database handles never go on the context.
- Return errors instead of writing responses where the router's `ErrorHandler`
  can map them (`anduin.Forbidden`, `anduin.NotFound`, domain sentinels). Use
  `anduin.Redirect` for a policy redirect such as a signed-out visitor.
- Keep policy out of `domains/` and `services/`; middleware decides whether a
  request may proceed, not whether a domain rule holds.

## Checklist

- [ ] Middleware has the `anduin.Middleware` shape and a distinct constructor
      when it has dependencies.
- [ ] Global middleware registered with `router.Use` in `app`;
      per-route middleware passed after the handler.
- [ ] Authentication runs before authorization, and identity is read from the
      database-backed viewer, not the cookie.
- [ ] Errors go to the router `ErrorHandler`; redirects use `anduin.Redirect`.
- [ ] `andurel verify` passes.

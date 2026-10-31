# Changelog

All notable changes to the standalone Andurel HTTP module are documented here.

## Unreleased

## 0.7.0 - 2026-10-31

### Added

- `Router.Route(r)` and `Host.Route(r)` register handlers for a `pkg/routing`
  route value on the host the route declares, so `routing.Host(...)` decides the
  domain. `Host.Route` panics when the route declares a different host. The
  returned `RouteRegistrar` has `Get`, `Post`, `Put`, `Patch` and `Delete`.
  `Router.Get` and the other path-string methods keep working.
- `Host.FallbackToPrimary()` checks the host's own routes first and uses the
  primary routes only when none match. `Host.Isolate()` answers unmatched
  requests on the host with 404.
- `Host.Use(mw...)` adds middleware for requests the host serves from its own
  routes. It runs after the global middleware and before per-route middleware,
  and not for requests that fall back to the primary routes.
- `Host.Mount(prefix, handler)` mounts an `http.Handler` for every method on
  `prefix` and `prefix/*`.
- `SkipCSRFForHosts(names...)` is a `CSRFConfig.Skipper` that skips CSRF checks
  for requests resolved to the named hosts.
- `anduintest.WithRouterOptions(...)` passes `anduin.Option`s to the test
  router, for example `anduin.WithHosts`.
- `SafeLocalPath(target)` reports whether a redirect target is a real path on
  this site. It normalises the target the way browsers do (trims ASCII
  whitespace and control characters, drops tabs, newlines and carriage returns,
  reads `\` as `/`) and rejects `//` prefixes, schemes, hosts, and empty or
  relative targets. `/\t/evil.example` and `/\evil.example` are rejected.
- `RedirectLocal(ctx, target, fallback, status)` redirects to `target` when it is
  a safe local path and to `fallback` otherwise. The redirect uses the normalised
  target. Use it for "return to where you were" after sign-in.

### Changed

- `Router.Handler()` panics when a non-primary host has routes but no fallback
  policy. Call `Host(name).FallbackToPrimary()` or `Host(name).Isolate()`. A host
  with no routes needs no call and keeps falling back to the primary routes.
- `Router.Host(routing.HostPrimary)` and `Router.Host("")` return the primary
  route table, so routes registered through them are served the same way as
  `Router.Get`.
- `StaticFS` and the router's `*` wildcard use `routing.WildcardParam` for the
  wildcard name. Requires routing v0.9.0, which also makes `/` match only `/`.

## 0.6.2 - 2026-10-30

### Changed

- Require published routing v0.8.0 and telemetry v0.2.3 without local replacements, so standalone builds use the released route and privacy fixes.

### Fixed

- `WithTelemetry` replaces the `url.path` placeholder started by
  `telemetry.WrapHandler` with the matched route template on the span and labels
  metrics with that route, so a secret path parameter never reaches traces or
  metric labels. `WrapHandler` now starts the span with `/[redacted]`, so a
  request that never reaches the telemetry middleware is still redacted.
- `Logger` and the default `ErrorHandler` debug record log the matched route
  pattern instead of the raw request path. An unmatched request logs an empty
  route, so a capability token in a URL never reaches logs.

### Tests

- Cover a custom JSON `ErrorHandler` receiving `ErrMethodNotAllowed` with
  `StatusCode` 405 and the `Allow` header `http.ServeMux` set, so the
  method-not-allowed manual action's JSON handler guidance is regression-proof.

## 0.6.1 - 2026-10-28

### Fixed

- Require `pkg/routing v0.7.0`, the first release providing `Hosts.Resolve`,
  so standalone consumers compile without an additional routing requirement.

## 0.6.0 - 2026-10-28

### Added

- `AddVary` appends a token to the `Vary` response header unless the header
  already lists it, comparing case-insensitively across comma-separated values.
  `Compress` uses it to keep `Accept-Encoding` merged with other middleware.
- `RequestHost(ctx)` reports the `routing.HostName` the router resolved for the
  request, for host-aware global middleware. Unknown authorities and contexts
  without router state report `routing.HostPrimary`; a configured host is
  reported even when it has no registered routes.
- `ErrMethodNotAllowed` is passed to the `ErrorHandler` when a route matches the
  path but not the request method. The `Allow` header set by `http.ServeMux` is
  preserved, and `StatusCode` reports 405.
- `anduintest.WithHosts` installs a host registry on the test router. A
  `Header{Name: "Host"}` request option sets `req.Host` so tests can target a
  named virtual host (Go's client ignores an explicit `Host` header).

### Changed

- Router host matching is case-insensitive and now shares
  `routing.Hosts.Resolve` instead of a private index, so an uppercase `Host`
  header dispatches to the configured host.
- A ServeMux-generated 405 now runs through the `ErrorHandler` instead of
  bypassing it. Without an `ErrorHandler` the response is still a 405 carrying
  `Allow`.

### Fixed

- `StaticFS` and `ServeContent` merge `Vary: Accept-Encoding` with existing
  values instead of replacing them, so the `Origin` CORS adds survives. A
  conditional `304 Not Modified` now carries `Vary` as well.

## 0.5.0 - 2026-10-27

### Added

- `Blob` writes a body with an explicit content type for responses that are
  neither JSON, text, nor HTML.
- `ServeContent` exports the ETag / conditional-GET handling `StaticFS` uses: it
  derives the same ETag format from the body, sets the content type from the
  file extension, and answers a matching `If-None-Match` with
  `304 Not Modified`. `SitemapURL.LastMod` adds the optional `<lastmod>` W3C
  datetime, omitted when zero.
- `Compress` gzips responses as an `http.Handler` wrapper, so it can sit outside
  `Router.Handler()` and cover `ErrorHandler` pages too. It skips bodies under
  1 KiB, already-encoded responses, `HEAD`, upgrades and event streams, and
  adds `Vary: Accept-Encoding`.
- `ErrNotFound` is passed to the `ErrorHandler` when no route matches a
  request, so apps can render their themed not-found page instead of
  `http.ServeMux`'s plain-text 404. `StatusCode` reports it as 404. Method
  mismatches still return 405.
- `anduintest.WithErrorHandler` installs an `ErrorHandler` on the test router
  so tests can exercise mapped errors.

### Changed

- `Sitemap` omits an empty `<changefreq>` element instead of writing it empty.

## 0.4.1 - 2026-10-26

### Changed

- Raise the minimum Go version from 1.27.1 to 1.27.3, the release that CI and
  `govulncheck` use, so consumers pick up the standard-library security fixes.

## 0.4.0 - 2026-10-26

### Added

- `WithHosts` installs the explicit `*routing.Hosts` registry used to dispatch
  requests to named hosts, replacing the package-level `routing.ConfigureHosts`
  lookup.

### Changed

- Bump `golang.org/x/net` to v0.60.0 for GO-2026-6617, GO-2026-6612,
  GO-2026-6611, GO-2026-6610, and GO-2026-6603, and `google.golang.org/grpc`
  to v1.83.2 for GO-2026-6443.

## 0.3.0 - 2026-10-26

### Added

- `StaticFS` serves an `fs.FS` for the route wildcard `path`, with the same
  path cleaning, 404 handling, content types and optional `Immutable` ETag /
  cache headers previously generated in application code. It returns
  `(Handler, error)` so an ETag walk failure other than a missing root surfaces
  at startup, and its explicit content-type table keeps fonts typed on images
  without `/etc/mime.types`.
- `RobotsTxt` and `Sitemap` build the `robots.txt` and sitemap XML responses.
- `Health` wraps a health checker with the generated 200/503 responses.
- `RateLimit` counts attempts per route and client IP through a `Counter`;
  `cache.Store` satisfies the interface.
- `SkipCSRFForBearer` is the bearer-token CSRF skipper for API requests.
- `DefaultCORS`, `DefaultCSRF`, and `MergeOrigins` carry the application CORS
  and CSRF policy that used to live in the runtime template.

## 0.2.1 - 2026-10-20

### Changed

- The `Shutdowner` doc comment no longer says `Server` shuts down application
  components; since 0.2.0 `Server.Shutdown` stops only the listener.

## 0.2.0 - 2026-10-20

### Changed

- `NewServer(ctx, host, port, handler, options...)` drops the unused
  environment argument and the shutdowner list, and `Server.Start()` drops its
  unused context and environment arguments. The process owns shutdown order;
  `Server.Shutdown` only stops the listener.

### Removed

- `Server.Shutdowners`. It was never shut down by `Server.Shutdown`.

## 0.1.0 - 2026-10-19

### Added

- Initial `anduin` HTTP package: a thin `net/http` layer over `http.ServeMux`
  (Go 1.22+ method/wildcard patterns).
- `Handler` (`func(context.Context) error`), `Middleware`, and `ErrorHandler`.
- `Router` with named virtual hosts, per-route middleware, and an `http.Handler`
  adapter (`Router.Handler`) so a test and a production process use the same
  router.
- Request helpers (`Request`, `PathValue`, `Query`, `QueryInt`, `DecodeJSON`,
  `DecodeForm`) and response helpers (`JSON`, `Text`, `HTML`, `NoContent`,
  `Redirect`).
- `HTTPError` and `Error`/`NotFound`/`BadRequest` constructors plus
  `StatusCode`.
- Generic middleware: `Recover`, `Logger`, `Telemetry`, `CORS`, `CSRF`.
- Trusted-proxy client IP resolution: `RealIP` middleware, `ClientIP`, and
  `ParseTrustedProxies`. `X-Forwarded-For` is only honoured when the direct
  peer is a trusted proxy; otherwise `ClientIP` is the `RemoteAddr` host.
- The `Server` listener and lifecycle merged in from the retired `pkg/server`
  module.
- `github.com/mbvlabs/andurel/pkg/anduin/anduintest` for handler-level tests.

### Notes

- `Router` translates the `pkg/routing` route model's `:param` segments into
  `http.ServeMux` `{param}` wildcards, so `pkg/routing` stays unchanged.

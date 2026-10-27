# Changelog

All notable changes to the standalone Andurel HTTP module are documented here.

## Unreleased

### Added

- `ErrNotFound` is passed to the `ErrorHandler` when no route matches a
  request, so apps can render their themed not-found page instead of
  `http.ServeMux`'s plain-text 404. `StatusCode` reports it as 404. Method
  mismatches still return 405.
- `anduintest.WithErrorHandler` installs an `ErrorHandler` on the test router
  so tests can exercise mapped errors.

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

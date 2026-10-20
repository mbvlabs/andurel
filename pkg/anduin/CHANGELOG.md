# Changelog

All notable changes to the standalone Andurel HTTP module are documented here.

## Unreleased

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

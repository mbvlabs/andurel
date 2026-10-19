# Changelog

All notable changes to the standalone Andurel telemetry module are documented here.

## Unreleased

## 0.2.0 - 2026-10-19

### Changed

- `From` now takes `*http.Request` (it previously took an `echo.Context` via the removed `HTTPContext` interface). It writes the child span context back onto the request. Handlers wired through `pkg/anduin` can also use `Start(ctx, name)`, since the telemetry middleware already attaches the pipeline to the request context.

## 0.1.0 - 2026-09-15

### Added

- OpenTelemetry logging, tracing, and metrics with positional `New` and `With*` options.
- Console stdout exporters for local development without an OTLP collector.
- Context-first `Start`, `Set`, `Fail`, and slog-shaped `Info` / `Error` / `Warn` / `Debug`.
- `From(*echo.Context, name)` for controllers; writes the child span context back onto the request.
- `WrapHandler` takes `*Telemetry` and passes both tracer and meter providers into otelhttp. HTTP server metrics come from otelhttp (`http.server.request.duration` and request/response body sizes). `SetHTTPRoute` labels `http.route` on the span and on otelhttp metrics.

### Fixed

- `MeterProvider()` returns a true nil interface when metrics are disabled, so Postgres instrumentation does not panic on a typed-nil SDK provider.
- Console log source points at the application caller instead of `handle.go`. Span-complete lines no longer report `console.go` as the source.

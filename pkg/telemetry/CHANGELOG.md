# Changelog

All notable changes to the standalone Andurel telemetry module are documented here.

## Unreleased

### Changed

- Console `span complete` lines log at debug, so they stay off at the default info level.
- One data API: slog-shaped kv on `Start`, `Set`, and `Info` / `Error` / `Warn` / `Debug`. Removed `Handle`, `Span.String` / `Int` / `Bool` / `Fail`, and the `*Context` log names.
- `Start`, `Set`, `Fail`, and log helpers take `context.Context`. Controllers enter with `From(*echo.Context, name)`, which starts the named span and writes the child context back onto the request. Add attributes with `Set`.

### Fixed

- `MeterProvider()` returns a true nil interface when metrics are disabled, so Postgres instrumentation does not panic on a typed-nil SDK provider.
- Console log source points at the application caller instead of `handle.go`. Span-complete lines no longer report `console.go` as the source.

## 0.1.0 - 2026-09-10

### Added

- OpenTelemetry logging, tracing, and metrics with positional `New` and `With*` options.
- Console stdout exporters for local development without an OTLP collector.
- Context-first `Start`, `Set`, `Fail`, and slog-shaped `InfoContext` / `ErrorContext` APIs.
- `From(ctx)` handle bound to the same API.
- HTTP helper `WrapHandler` plus request and route recording without process-wide OTel globals.

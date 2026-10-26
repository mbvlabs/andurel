# Changelog

All notable changes to the standalone Andurel lifecycle module are documented here.

## Unreleased

## 0.1.0 - 2026-10-26

### Added

- Initial standalone lifecycle module with `Stack`, which closes resources in
  reverse order and joins errors, and `Run`, which handles SIGINT/SIGTERM and
  stops the process with a configurable timeout.
- `NotifyContext`, which returns a context cancelled on SIGINT/SIGTERM, and a
  `Run` that passes a signal-aware context to its start function.
- `WithBlockingStart` supports blocking servers with concurrency-safe shutdown
  and waits for the serve function to exit within the shutdown timeout.

### Fixed

- `Run` finishes startup before stopping resources on cancellation, preventing
  startup from acquiring resources after shutdown has already returned.

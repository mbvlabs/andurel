# Changelog

All notable changes to the standalone Andurel server module are documented here.

## 0.3.0 - 2026-09-05

### Added

- `DefaultConfig` for bounded HTTP timeouts.
- `ServerOptions.Validate`. Zero disables a timeout; negative values are rejected.

### Changed

- `New` applies `DefaultConfig` when timeouts are omitted.
- `New` copies supplied shutdown hooks before appending the HTTP server.

## 0.2.0 - 2026-09-02

### Added

- `WithTimeouts` for idle, read, and write timeouts.

### Removed

- Built-in timeout defaults in `New`. Applications supply timeout values.

## 0.1.0 - 2026-08-20

### Added

- Initial standalone HTTP server with graceful shutdown and configurable read, write, and idle timeouts.
- Support for registering additional shutdown hooks.

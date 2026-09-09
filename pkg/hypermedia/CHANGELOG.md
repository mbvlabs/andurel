# Changelog

All notable changes to the standalone Andurel hypermedia module are documented here.

## Unreleased

### Changed

- Minimum supported Go version is now 1.26.0.

## 0.2.2 - 2026-09-07

### Changed

- Minimum supported Go version is now 1.27.0.

## 0.2.1 - 2026-09-01

### Changed

- Reformatted source with golines; no functional changes.

## 0.2.0 - 2026-08-24

### Removed

- Dependency on the removed `pkg/request` module.
- `ResolveBackURL`; request-scoped back URLs now live in application-owned `router/appctx`.

## 0.1.0 - 2026-08-20

### Added

- Initial standalone hypermedia module with templ page and fragment rendering.
- Datastar element patches, signal updates, SSE streaming, script execution, and broadcaster helpers.

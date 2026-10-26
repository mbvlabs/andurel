# Changelog

## Unreleased

### Changed

- Raise the minimum Go version from 1.27.1 to 1.27.3, the release that CI and
  `govulncheck` use, so consumers pick up the standard-library security fixes.

## 0.2.0 - 2026-10-20

### Added

- `Store.Close`. The Redis driver closes its client and connection pool; the
  memory driver stops its background goroutines. Applications close the store
  on shutdown so Redis connections are not leaked.

## 0.1.0 - 2026-10-19

### Added

- `Store` interface (`Get`, `Put`, `Forget`, `Flush`, `Increment`) with
  JSON-encoded values so drivers behave identically.
- `Remember` and `Get` typed helpers with per store+key singleflight, so a
  cold key loads once under a thundering herd and loader errors never cache.
- `memory` driver (otter, in-process) and `redis` driver (shared across
  replicas; fixed-window `Increment` for rate limiting).
- `Config`/`New` with `Namespace` isolation; `Flush` only touches namespaced
  keys and never runs `FLUSHDB`.

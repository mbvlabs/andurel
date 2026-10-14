# Changelog

## Unreleased

### Added

- `Store` interface (`Get`, `Put`, `Forget`, `Flush`, `Increment`) with
  JSON-encoded values so drivers behave identically.
- `Remember` and `Get` typed helpers with per store+key singleflight, so a
  cold key loads once under a thundering herd and loader errors never cache.
- `memory` driver (otter, in-process) and `redis` driver (shared across
  replicas; fixed-window `Increment` for rate limiting).
- `Config`/`New` with `Namespace` isolation; `Flush` only touches namespaced
  keys and never runs `FLUSHDB`.

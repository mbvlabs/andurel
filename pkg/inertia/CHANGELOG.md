## Unreleased

### Changed

- The HTTP app renderer is always an SSR HTTP client. Node process ownership
  belongs to a separate `cmd/ssr` entrypoint (Laravel-style), not `cmd/app`.
- Replaced `SSRMode` / `WithSSRMode` with this split. Per-response `WithSSR()`
  remains the opt-in for SSR rendering.

### Removed

- `SSRMode`, `SSRDisabled`, `SSRExternal`, `SSRManaged`, `WithSSRMode`, and
  `WithAppManagedSSR`.

## 0.3.1 - 2026-09-07

### Changed

- Minimum supported Go version is now 1.27.0.

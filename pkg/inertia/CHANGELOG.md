## Unreleased

### Changed

- Public init is `NewRenderer` + `NewSSRRuntime` on `Renderer` and `SSRRuntime`.
- Renamed `ManagedConfig` / `ManagedRuntime` to `SSRConfig` / `SSRRuntime`.
- HTTP transport settings used by the renderer client are `SSRClientConfig`
  (also nested as `SSRConfig.HTTP`).
- Construction and single-use private setup helpers are inlined into
  `NewRenderer`, `NewSSRRuntime.Start`, `Page`, `Middleware`, and `PageScript`.

### Removed

- `New` (use `NewRenderer`).
- `Config`, `DefaultConfig`, `RendererConfig`, `DefaultRendererConfig`,
  `DefaultSSRConfig`, and unexported `defaultSSRClientConfig`.
- `WithSSRClientConfig`, `WithSSRRuntime`, `WithSSRBundle`, `WithSSRStartupTimeout`.
- `Renderer.Start` / `Renderer.Shutdown`.
- `ManagedConfig`, `DefaultManagedConfig`, `ManagedRuntime`, `NewManagedRuntime`,
  and the old HTTP-facing `SSRConfig` / `DefaultSSRConfig` / `WithSSRConfig`
  names (process config is now `SSRConfig`; defaults live in `NewSSRRuntime`).

## 0.4.0 - 2026-09-07

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

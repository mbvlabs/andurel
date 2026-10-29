## Unreleased

## 0.13.0 - 2026-10-29

### Added

- `RendererConfig.SSRDefault` and `WithSSRDefault` server-render every initial
  document response, so an application no longer has to call
  `PageBuilder.SSR()` on each page. The zero value keeps the existing per-page
  opt-in, and Inertia JSON (`X-Inertia`) visits are unaffected.
- `PageBuilder.CSR()` opts one page out of the renderer-wide SSR default.
  `PageBuilder.SSR()` still wins when both are set.
- `Renderer.Readiness` and `Renderer.Health` report whether SSR is required and
  the configured sidecar answers a bounded `/health` probe. Readiness is
  computed per call, so losing the sidecar after readiness makes the
  application unready and restarting it makes the application ready again.
  `SSRStatus` is the stable reason (`ready`, `disabled`, `csr_fallback`,
  `unavailable`, `unsupported`); the Vite development transport and custom
  renderers without a health endpoint are `unsupported`.
- `SSRRuntime.Health` re-probes the managed sidecar so a launcher can confirm
  the intended child process is ready.

### Changed

- Error pages stay client-rendered unless `ErrorMapping.SSR`, `ValidationSSR`,
  or `FallbackSSR` opt in, even when the renderer defaults every other page to
  SSR. A failed SSR render therefore never retries SSR for the error page it
  caused.
- An unmapped 5xx document request without a configured `FallbackComponent` is
  answered by a built-in escaped HTML page with the status text only. It builds
  no Inertia page, so it cannot reference a missing component, recurse into SSR
  or leak handler details. JSON requests still pass the error to the router.

## 0.12.1 - 2026-10-28

### Fixed

- Require `pkg/anduin v0.6.1`, which provides `AddVary` and correctly requires
  `pkg/routing v0.7.0`, so standalone consumers compile without additional
  Andurel dependency requirements.

## 0.12.0 - 2026-10-28

### Added

- `PageBuilder.URL` overrides the page object's `url` in both the JSON
  (`X-Inertia`) and full-document/SSR responses, so a handler that renders after
  a POST can point the browser at a GET-able route.

## 0.11.0 - 2026-10-27

### Added

- `ErrorMapping.SSR` opts a mapped error page into server-side rendering, and
  `NewErrorHandler` with `ErrorHandlerOptions` does the same for the validation
  and fallback pages. A configured fallback page renders unmapped 5xx errors
  (including a fail-fast SSR failure) as a themed response instead of the
  router's plain-text default; it stays client-rendered when the error came from
  SSR itself. `ErrorHandler` keeps its existing `...ErrorMapping` signature.

### Fixed

- `resolveSSRBundle` falls back to the embedded bundle when a configured
  `INERTIA_SSR_BUNDLE` is an absolute path outside the project, and the error
  now reports that the embedded fallback was tried.

## 0.10.1 - 2026-10-26

### Changed

- Raise the minimum Go version from 1.27.1 to 1.27.3, the release that CI and
  `govulncheck` use, so consumers pick up the standard-library security fixes.

## 0.10.0 - 2026-10-26

### Changed

- Bump `golang.org/x/net` to v0.60.0 for GO-2026-6617, GO-2026-6612,
  GO-2026-6611, GO-2026-6610, and GO-2026-6603, and `google.golang.org/grpc`
  to v1.83.2 for GO-2026-6443.

### Fixed

- Page JSON is HTML-escaped in both the JSON response and `PageScript`, so a
  prop containing `</script>` or `<!--` cannot break out of the document.

## 0.9.0 - 2026-10-26

### Added

- `ErrorHandler` and `MapError` translate handler errors into JSON or Inertia
  responses: field errors become a 422 page, mapped sentinels render the
  configured pages, and unmapped errors keep their status without leaking 5xx
  details.
- `PageBuilder.Errors` sets the validation errors prop from an error and reports
  whether it carried field errors.

## 0.8.1 - 2026-10-20

### Changed

- Requires `pkg/anduin` v0.2.1.

## 0.8.0 - 2026-10-20

### Changed

- `NewRenderer(cfg RendererConfig, options...)` and
  `NewSSRRuntime(cfg SSRRuntimeConfig, options...)` take a config struct
  instead of seven positional arguments, so call sites name every setting.

## 0.7.0 - 2026-10-19

### Changed

- The renderer no longer depends on Echo. `Page`, `Redirect`, `Location`,
  `FreshRedirect`, `Middleware`, and every request-scoped callback
  (`SharedProvider`, `VersionProvider`, `ReflashHandler`, `FlashProvider`,
  `Resolver`, `PropResolver.ResolveInertiaProp`, `ScrollMetadataResolver`, and
  lazy prop funcs) now take a plain `context.Context`. Request and response
  access goes through `github.com/mbvlabs/andurel/pkg/anduin`.
- `ParseRequest` takes a `context.Context`; `ParseHTTPRequest` is unchanged.
- `Renderer.Middleware` returns an `anduin.Middleware` and captures the response
  through `anduin.ResponseWriter` / `anduin.ReplaceResponseWriter`.
- Package doc comments refer to `anduin` rather than Echo.

### Removed

- The `github.com/labstack/echo/v5` dependency.

## 0.6.1 - 2026-10-05

### Changed

- Production Vite `<link>` and `<script type="module">` tags include
  `crossorigin` so a page on a secondary host can load assets from the
  origin in `buildPathURL` (pass `routes.ViteBuild.FullURL()`).

## 0.6.0 - 2026-09-17

### Changed

- Minimum supported Go version is now 1.27.1.

### Added

- In `development`, `NewRenderer` posts SSR requests to the Vite origin's
  `/__inertia_ssr` endpoint (from `ViteDevURL`) instead of `{SSRURL}/render`.

## 0.5.0 - 2026-09-09

### Added

- `ValidateSSRListen` and `ValidateSSRClient` for listen vs client URL rules.
- `WithSSRLogger`, `WithSSRStdout`, and `WithSSRStderr` on `NewSSRRuntime`.
- Optional `fs.FS` bundle fallback on `NewSSRRuntime` for CNB images that embed
  `assets/dist/ssr/ssr.js` but do not keep it on disk.

### Changed

- Split Node bind from the Go render client: `NewSSRRuntime` takes a listen URL
  (`INERTIA_SSR_LISTEN`; IP or localhost, including `0.0.0.0` / `::`).
  `NewRenderer` / `NewHTTPRenderer` take the client URL (`INERTIA_SSR_URL`; any
  http(s) host, including service DNS).
- `NewSSRRuntime` takes positional process arguments instead of `SSRConfig`.
- `NewRenderer` and `NewHTTPRenderer` take `ssrURL`, timeout, and max response
  bytes as arguments instead of `SSRClientConfig`.
- Health and shutdown probe loopback on the listen port when the bind host is
  unspecified. They do not probe `0.0.0.0`, `::`, or `INERTIA_SSR_URL`.

### Removed

- `SSRConfig`, `SSRConfig.Validate`, and `SSRClientConfig`.
- `SSRRuntime.Renderer`.
- The runtime `Enabled` flag. Omit `Start` to leave the process unstarted.

## 0.4.2 - 2026-09-09

### Changed

- Minimum supported Go version is now 1.26.0.

## 0.4.1 - 2026-09-08

### Changed

- Public init is `NewRenderer` + `NewSSRRuntime` on `Renderer` and `SSRRuntime`.
- Renamed `ManagedConfig` / `ManagedRuntime` to `SSRConfig` / `SSRRuntime`.
- HTTP transport settings used by the renderer client are `SSRClientConfig`
  (also nested as `SSRConfig.HTTP`).
- Construction and single-use private setup helpers are inlined into
  `NewRenderer`, `NewSSRRuntime.Start`, `Page`, `Middleware`, and `PageScript`.
- `NewRenderer` now takes positional arguments: `containerID`, `buildPathURL`,
  `entryPoint`, `viteDevURL`, and `SSRClientConfig`. Required Inertia protocol
  settings are no longer provided through options.
- `NewSSRRuntime` no longer fills defaults for `Executable`, `StartupTimeout`,
  `MinimumMajor`, or `SSRClientConfig` fields. These must be supplied by the
  caller; `Logger`, `Stdout`, and `Stderr` still default to `slog.Default`,
  `os.Stdout`, and `os.Stderr`.
- `Page` now returns a `*PageBuilder` for method chaining. Per-page options
  are set through builder methods (`SSR()`, `Status()`, `ValidationErrors()`,
  etc.) instead of functional options.

### Removed

- `New` (use `NewRenderer`).
- `Config`, `DefaultConfig`, `RendererConfig`, `DefaultRendererConfig`,
  `DefaultSSRConfig`, and unexported `defaultSSRClientConfig`.
- `WithSSRClientConfig`, `WithSSRRuntime`, `WithSSRBundle`, `WithSSRStartupTimeout`.
- `Renderer.Start` / `Renderer.Shutdown`.
- `ManagedConfig`, `DefaultManagedConfig`, `ManagedRuntime`, `NewManagedRuntime`,
  and the old HTTP-facing `SSRConfig` / `DefaultSSRConfig` / `WithSSRConfig`
  names (process config is now `SSRConfig`; defaults live in `NewSSRRuntime`).
- `WithContainerID`, `WithBuildPathURL`, `WithEntryPoint`, `WithViteDevURL`,
  `WithSSRURL`, `WithSSRRequestTimeout`, and `WithSSRMaxResponseBytes` options
  (replaced by positional arguments on `NewRenderer`).
- `PageOption`, `WithSSR`, `WithStatus`, `WithValidationErrors`,
  `WithHistoryEncryption`, `WithHistoryClear`, `WithPreserveFragment`,
  and `WithFlash` (replaced by builder methods on `*PageBuilder`).

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

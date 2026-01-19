# PR3: Observability - Replace `otelecho` with `otelhttp` (Plumb Through `router.New`)

Purpose
- Replace Echo-specific OTEL middleware (`otelecho`) with net/http instrumentation (`otelhttp`).
- Make it configurable but default to always-on.
- Ensure we can still label spans with the matched route pattern.

Context / decisions
- We determined `otelecho` targets Echo v4 and does not appear to have an Echo v5 variant.
- We want net/http instrumentation as the foundation.
- Toggle should be passed into `router.New(...)` (not via env vars).

Scope
- Templates:
  - `layout/templates/router_router.tmpl`
  - `layout/templates/cmd_app_main.tmpl`
  - (potentially) `layout/templates/go_mod.tmpl` if dependencies are pinned there
- Out of scope: route registration changes, CSRF changes.

Detailed changes

1) Remove `otelecho` usage
- In `layout/templates/router_router.tmpl`:
  - Remove `otelecho.Middleware(config.ServiceName)` from `SetupGlobalMiddleware`.
  - Remove the `otelecho` import.

2) Add otelhttp instrumentation wrapper
- Add imports:
  - `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
- Update router struct to expose both Echo handler and an http.Handler:
  - `Handler *echo.Echo` (existing)
  - `HTTPHandler http.Handler` (new)

3) Plumb a flag through `router.New`
- Update `router.New(...)` signature to accept e.g.
  - `enableHTTPInstrumentation bool`
- Behavior:
  - if `enableHTTPInstrumentation` is true:
    - `HTTPHandler = otelhttp.NewHandler(Handler, "http")` (name TBD)
  - else:
    - `HTTPHandler = Handler`

4) Preserve route naming in spans
- Add an Echo middleware that sets route attributes after routing.
- Use Echo v5 APIs:
  - `c.RouteInfo().Path` for the matched route pattern.
- Set attributes on the active span (and/or metric labels).
- This middleware should be independent of otelhttp; it just annotates.

5) Wire server startup to use `HTTPHandler`
- In `layout/templates/cmd_app_main.tmpl`:
  - pass `rtr.HTTPHandler` into server creation (or StartConfig) rather than `rtr.Handler`.

6) Dependencies
- Ensure scaffold `go.mod` includes `otelhttp` already (it likely does in telemetry templates); if not, add it.

Acceptance criteria
- Scaffold compiles without `otelecho`.
- HTTP requests are traced via otelhttp.
- Spans include route pattern (best-effort) using Echo v5 `RouteInfo`.

Notes
- Exact span naming conventions can be refined later; in this PR focus on correctness.

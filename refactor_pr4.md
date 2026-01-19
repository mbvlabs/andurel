# PR4: Route Registration - Use `AddRoute` + Aggregate Errors

Purpose
- Echo v5 route registration methods (`GET`, `Add`, etc.) return `RouteInfo` values and can no longer be mutated (`.Name = ...`).
- We want stable route names (for reverse routing and observability) and Rails-like startup failures.
- Implement `AddRoute(echo.Route{ Name: ... })` everywhere and collect all registration errors.

Scope
- Templates:
  - All `layout/templates/router_connect_*_routes.tmpl`
  - `generator/templates/route_registration.tmpl`
  - Any other templates that call `.Add(...).Name = ...`.
- Also update `router.Router.RegisterCtrlRoutes` to return `error` (and bubble it).

Detailed changes

1) Convert all route registrations to `AddRoute`
- Pattern today (v4):
  - `handler.Add(method, path, handlerFn).Name = routes.Foo.Name()`
- New pattern (v5):
  - `_, err := handler.AddRoute(echo.Route{ Method: method, Path: routes.Foo.Path(), Name: routes.Foo.Name(), Handler: controllerFn })`

2) Make registration functions return errors
- Each `registerXRoutes(handler *echo.Echo, ctrl controllers.X)` should:
  - return `error`
  - collect per-route errors into `[]error`
  - return `errors.Join(errs...)` (Go 1.20+)

3) Aggregate at the router level
- Update `func (r *Router) RegisterCtrlRoutes(...)`:
  - returns `error`
  - calls each registration function and collects the errors
  - returns `errors.Join(...)`

4) Bubble the error to main
- In `layout/templates/cmd_app_main.tmpl`:
  - check error from `rtr.RegisterCtrlRoutes(...)` and return it

5) Keep typed routes
- Do not remove or change `internal/routing` or `router/routes`.
- Keep URL building typed and independent of Echo.

Acceptance criteria
- Scaffold compiles with Echo v5 route naming.
- Startup fails with an aggregated error if any route fails to register.

Notes
- `echo.Route` supports `Middlewares` too; in this PR keep it nil unless currently used.

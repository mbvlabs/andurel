# PR5: Response API Migration - Use `http.ResponseController` + `echo.UnwrapResponse`

Purpose
- Echo v5 changes `c.Response()` to return `http.ResponseWriter`.
- Existing scaffold code uses Echo v4 response internals (`c.Response().Writer`, `c.Response().Flush()`, `c.Response().Status`).
- We want to standardize on Go's `http.ResponseController` for streaming/SSE and use `echo.UnwrapResponse` when we need status/size.

Scope
- Hypermedia templates:
  - `layout/templates/framework_elements_hypermedia_broadcaster.tmpl`
  - `layout/templates/framework_elements_hypermedia_sse.tmpl`
  - `layout/templates/framework_elements_hypermedia_core.tmpl`
- Router middleware metrics/logging:
  - `layout/templates/router_middleware_middleware.tmpl`

Detailed changes

1) SSE/broadcaster: stop using `c.Response().Writer`
- Replace:
  - `c.Response().Writer` -> `c.Response()` (still implements `io.Writer`)
  - `http.NewResponseController(c.Response().Writer)` -> `http.NewResponseController(c.Response())`

2) Flushing
- Replace `c.Response().Flush()` with:
  - `rc := http.NewResponseController(c.Response())`
  - `if err := rc.Flush(); err != nil { ... }`

3) Accessing status codes
- In `layout/templates/router_middleware_middleware.tmpl` (request metrics/logger), replace:
  - `statusCode := c.Response().Status`
- With:
  - `resp, err := echo.UnwrapResponse(c.Response())`
  - if `err == nil`, use `resp.Status`
  - otherwise fall back to `0` or `http.StatusOK` depending on current semantics.

4) Maintain behavior
- Ensure headers for SSE are set the same (Cache-Control, Content-Type, Connection for HTTP/1).

Acceptance criteria
- No remaining usage of `c.Response().Writer`, `c.Response().Flush()`, or `c.Response().Status` in templates.
- SSE works with Echo v5 response model.

Notes
- Avoid refactoring unrelated hypermedia logic.

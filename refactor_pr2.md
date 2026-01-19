# PR2: Echo v5 Imports + Handler Signatures (Templates Only)

Purpose
- Perform the mechanical bulk migration from Echo v4 to Echo v5 across templates:
  - imports: `github.com/labstack/echo/v4` -> `github.com/labstack/echo/v5`
  - middleware imports: `github.com/labstack/echo/v4/middleware` -> `github.com/labstack/echo/v5/middleware`
  - handler/middleware signatures: `echo.Context` -> `*echo.Context`

Scope
- Modify template files only (no generator runtime code changes):
  - `layout/templates/**/*.tmpl`
  - `generator/templates/**/*.tmpl`
- Out of scope: route registration refactor, otel changes, CSRF changes, response streaming changes.

Detailed steps

1) Replace imports
- In all `layout/templates/**/*.tmpl` and `generator/templates/**/*.tmpl`:
  - Replace `"github.com/labstack/echo/v4"` with `"github.com/labstack/echo/v5"`
  - Replace `echomw "github.com/labstack/echo/v4/middleware"` with `echomw "github.com/labstack/echo/v5/middleware"`

2) Replace handler signatures
- Replace all function parameters typed as `echo.Context` with `*echo.Context`.

Examples to update (non-exhaustive)
- Controllers:
  - `layout/templates/controllers_*.tmpl` (`func (X) Foo(etx echo.Context) error`)
  - `generator/templates/controller*.tmpl`
- Hypermedia:
  - `layout/templates/framework_elements_hypermedia_*.tmpl` (utility funcs currently take `echo.Context`)
- Renderer:
  - `layout/templates/framework_elements_renderer_render.tmpl` currently takes `echo.Context`
- Router middleware:
  - `layout/templates/router_middleware_*.tmpl` closures currently use `func(c echo.Context) error`.

3) Replace middleware callback signatures
- Any `Skipper`-like functions must be updated to `func(c *echo.Context) bool`.

4) Fix compilation fallout in templates (only what is required)
- Some APIs remain in v5 but on pointer receiver; after signature change you may need to update method calls accordingly.

Acceptance criteria
- No remaining template imports of `echo/v4`.
- `go test ./...` for this repo may still fail due to later PR work (route registration, otel, etc.), but templates should be mechanically consistent and ready for follow-up PRs.

Notes
- Do not attempt to refactor route naming or registration in this PR.
- Avoid reformatting unrelated areas.

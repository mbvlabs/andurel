# PR6: Cookies + Sessions - Remove Embedded Echo Context, Upgrade `echo-contrib/session`

Purpose
- Echo v5 makes `Context` a concrete struct; embedding/copying it is fragile.
- Our cookie/session helpers currently embed `echo.Context` in structs (`App`, `FlashMessage`).
- We want plain data structs and explicit read/write using `echo-contrib/session`.

Scope
- Templates:
  - `layout/templates/router_cookies_cookies.tmpl`
  - `layout/templates/router_cookies_flash.tmpl`
  - any call sites affected:
    - `layout/templates/router_middleware_auth.tmpl`
    - `layout/templates/router_middleware_middleware.tmpl`
    - controllers that call `cookies.AddFlash(...)` or `cookies.GetApp(...)`
- Also update scaffold dependency from echo-contrib session targeting v4 -> v5 API.

Detailed changes

1) Upgrade `echo-contrib/session` version
- In scaffold `go.mod` template(s):
  - ensure `github.com/labstack/echo-contrib/session` resolves to v0.50.0+ (Echo v5 support).
- Code/API change:
  - `session.Get(name, c echo.Context)` becomes `session.Get(name, c *echo.Context)`.

2) Refactor `cookies.App`
- In `layout/templates/router_cookies_cookies.tmpl`:
  - Remove `echo.Context` field from `type App struct`.
  - Keep only app state fields (UserID, IsAdmin, IsAuthenticated, etc.)
  - Update functions:
    - `CreateAppSession(c *echo.Context, user models.User) error`
    - `DestroyAppSession(c *echo.Context) error`
    - `GetApp(c *echo.Context) App`
  - Implementation should use `session.Get(...)` + `sess.Values[...]` + `sess.Save(c.Request(), c.Response())`.

3) Refactor `cookies.FlashMessage`
- In `layout/templates/router_cookies_flash.tmpl`:
  - Remove embedded context from `FlashMessage`.
  - Keep message fields only.
  - Update APIs to `*echo.Context`.

4) Update middleware that stores app/flash context
- Ensure these continue to work:
  - `mw.RegisterAppContext`: `c.Set(string(cookies.AppKey), cookies.GetApp(c))`
  - `mw.RegisterFlashMessagesContext`: `c.Set(string(cookies.FlashKey), flashes)`

Acceptance criteria
- No embedded Echo context in cookie structs.
- All cookie/session APIs accept `*echo.Context`.
- Scaffold builds against echo-contrib/session v0.50.0+.

Notes
- This PR should not attempt broader auth/session redesign.

# Changelog

All notable changes to the standalone Andurel kiks module are documented here.

## Unreleased

### Changed

- Bump `golang.org/x/net` to v0.60.0 for GO-2026-6617, GO-2026-6612,
  GO-2026-6611, GO-2026-6610, and GO-2026-6603, and `google.golang.org/grpc`
  to v1.83.2 for GO-2026-6443.

### Fixed

- `MaxAge` now applies to each cookie definition. A session or named cookie's
  lifetime no longer rejects a longer-lived signed or encrypted cookie.
- `NewJar` and `NewCookieStore` reject an encryption key that is not 16, 24, or
  32 bytes.

## 0.4.0 - 2026-10-26

### Changed

- `NewSession`, `Plain`, `Signed`, and `Encrypted` accept any `T`. Types that
  implement `Cookie` keep their custom codec; other types are encoded as JSON,
  and an empty payload decodes to the zero value.

## 0.3.1 - 2026-10-20

### Changed

- Requires `pkg/anduin` v0.2.1.

## 0.3.0 - 2026-10-19

### Changed

- Replaced `(*Jar).EchoMiddleware` with `(*Jar).Middleware`, which returns
  `anduin.Middleware` (`func(anduin.Handler) anduin.Handler`) and operates on
  the native `context.Context` request context. Handlers now use `anduin`
  `Redirect` / `NoContent` / `JSON` / `Text` / `HTML` helpers.
- `Middleware` now attaches the cookie bag to the handler `context.Context`
  (not the request context), so `kiks.Get`/`Set`/`Destroy` work after
  `anduin.Router` re-derives the request from the middleware context.
- Removed the `github.com/labstack/echo/v5` dependency.

## 0.2.0 - 2026-09-22

### Added

- `Bagged(def)` marks a Plain/Signed/Encrypted definition for the middleware
  bag. `NewSession` is always bagged. Unmarked named defs stay registered for
  type-keyed native access only.
- Native cookie API: `Read` / `Write` / `Clear` take `(ctx, jar, r|w)` and look
  up `T` on the jar. Bagged types return `ErrBagCookie`; bag API on native
  types returns `ErrNotBag`.

### Changed

- Corrupt-cookie recovery is narrow: only `securecookie.Error` decode failures
  and intentional payload/flash JSON unmarshal errors. Store/infra errors
  propagate from load middleware instead of clearing the session.
- `persist` returns errors; middleware fails the request on encode/save
  failures instead of swallowing them.
- Public store API is cookie-only (`NewCookieStore`). Database session types
  (`databaseStore`, `newDatabaseStore`, `sessionDB`) are unexported; `NewStore`,
  `DriverCookie`/`DriverDatabase`, and `SQLSessionDB` are removed from the
  public surface.
- `NewJar` applies the session `MaxAge` to jar codecs and store codecs.
  Private `databaseStore` no longer invents a 24h expiry when `MaxAge <= 0`.
- Flash cookies encode `[]FlashMessage` as JSON inside the encrypted envelope
  (hard cut; gob flash cookies are treated as corrupt).
- `Get` / `Exists` / `Set` / `Destroy` return `error` (`ErrNoBag`,
  `ErrUnknownType`). Missing cookie values are still `(zero, nil)` /
  `(false, nil)` — a wrong type arg (e.g. `Get[Cart]` vs jar `*Cart`) errors.
- Cookie definitions require `T Cookie` (`MarshalCookie` / `UnmarshalCookie`).
  Prefer pointer types (`*App`) so methods use pointer receivers. One Go type
  per jar registration.
- Renamed `Session` → `NewSession`, `New` → `NewJar(keys, store, defs...)`, and
  `Delete` → `Destroy`. Added `Exists[T]`.
- Session persistence goes through a `Store`; scaffolds use `CookieStore`.
- Flashes always live in a dedicated flash cookie (`{sessionName}_flash`),
  never in the AppCookie envelope and never in the sessions table.
- Middleware eager-loads only the session + flash cookie; bagged named cookies
  (`Plain` / `Signed` / `Encrypted` wrapped with `Bagged`) load lazily on
  first `Get` / `Exists`.

## 0.1.0 - 2026-09-17

### Added

- Initial standalone cookie and session module with named definitions
  (`Plain`, `Signed`, `Encrypted`, `Session`), type-keyed
  `Get`/`Set`/`Delete` on `context.Context`, and flash helpers.
- Cookie-only sessions: `New` takes keys and definitions; session values
  live in an encrypted cookie. Cookie value types (and session envelope
  types) are registered with `gob` when the jar is constructed.
- Echo middleware that loads the request bag, recovers corrupt cookies, and
  persists once using the flash render/redirect policy.

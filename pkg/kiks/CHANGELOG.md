# Changelog

All notable changes to the standalone Andurel kiks module are documented here.

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

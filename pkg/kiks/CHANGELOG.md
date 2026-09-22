# Changelog

All notable changes to the standalone Andurel kiks module are documented here.

## Unreleased

### Changed

- Cookie definitions require `T Cookie` (`MarshalCookie` / `UnmarshalCookie`).
  Prefer pointer types (`*App`) so methods use pointer receivers. Payload
  encoding no longer uses gob of `T`; only flash cookies still use gob.
- `SessionDB` matches `storage.Connection` Exec/QueryRow (`pgconn.CommandTag`,
  `pgx.Row`) so a pgx connection can be passed to `NewStore` directly.
- Removed `SessionsTableSQL`; apps own the sessions table migration for the
  database driver.

### Changed (prior)

- Renamed `Session` → `NewSession`, `New` → `NewJar(keys, store, defs...)`, and
  `Delete` → `Destroy`. Added `Exists[T]`.
- Session persistence goes through a `Store` (`CookieStore` default,
  `DatabaseStore` optional) selected via `NewStore(driver, keys, db)`.
- Flashes always live in a dedicated flash cookie (`{sessionName}_flash`),
  never in the AppCookie envelope and never in the sessions table.
- Middleware eager-loads only the session + flash cookie; named cookies
  (`Plain` / `Signed` / `Encrypted`) load lazily on first `Get` / `Exists`.

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

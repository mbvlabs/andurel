# Changelog

All notable changes to the standalone Andurel kiks module are documented here.

## Unreleased

### Changed

- `New` takes only keys and definitions; session values always live in an
  encrypted cookie. The SQL/`SessionStore` API is gone.
- `New` registers each cookie value type (and session envelope types) with
  `gob` when the jar is constructed. No package `init` registration.

## 0.1.0 - 2026-09-17

### Added

- Initial standalone cookie and session module with named definitions
  (`Plain`, `Signed`, `Encrypted`, `Session`), type-keyed
  `Get`/`Set`/`Delete` on `context.Context`, and flash helpers.
- `SessionStore` with a gorilla/securecookie cookie driver and a SQL driver.
- Echo middleware that loads the request bag, recovers corrupt cookies, and
  persists once using the flash render/redirect policy.

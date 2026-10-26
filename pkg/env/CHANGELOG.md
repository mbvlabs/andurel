# Changelog

All notable changes to the standalone Andurel env module are documented here.

## Unreleased

### Changed

- Raise the minimum Go version from 1.27.1 to 1.27.3, the release that CI and
  `govulncheck` use, so consumers pick up the standard-library security fixes.

## 0.1.0 - 2026-10-26

### Added

- Initial standalone env module with `Load` for `.env` files and a `Reader`
  with typed accessors for strings, integers, booleans, floats, durations, and
  comma-separated lists.

# Changelog

All notable changes to the standalone Andurel validation module are documented here.

## Unreleased

### Changed

- Minimum supported Go version is now 1.26.0.

## 0.1.2 - 2026-09-07

### Changed

- Minimum supported Go version is now 1.27.0.
- Adopted Go 1.27 standard-library simplifications for reflected types and map copying.

## 0.1.1 - 2026-09-01

### Changed

- Reformatted source with golines; no functional changes.

## 0.1.0 - 2026-08-20

### Added

- Initial standalone validation module with structured field errors, rule builders, and helpers for common constraints.
- Support for validating structs, maps, and nested values with composable rules.

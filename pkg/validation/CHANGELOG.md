# Changelog

All notable changes to the standalone Andurel validation module are documented here.

## Unreleased

## 0.3.0 - 2026-10-26

### BREAKING

- Removed the deprecated `Validate` function and `Validatable` interface. Call
  a value's `Validate` method directly and join errors with `validation.Join`.

## 0.2.0 - 2026-10-26

### Added

- `Join` combines validation errors and preserves non-validation errors;
  `Field` builds a single field error; and `ValidationBuilder.Email` validates
  an address with `net/mail`.

### Deprecated

- `Validate` and `Validatable`; call a value's `Validate` method directly.

### Fixed

- `Join` collects every field error beneath single-error wrappers and preserves
  the original wrapped error trees, including non-validation sentinels.

## 0.1.4 - 2026-09-17

### Changed

- Minimum supported Go version is now 1.27.1.

## 0.1.3 - 2026-09-09

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

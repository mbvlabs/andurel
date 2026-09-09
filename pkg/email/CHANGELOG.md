# Changelog

All notable changes to the standalone Andurel email module are documented here.

## Unreleased

## 0.3.2 - 2026-09-09

### Changed

- Minimum supported Go version is now 1.26.0.

## 0.3.1 - 2026-09-07

### Changed

- Minimum supported Go version is now 1.27.0.

## 0.3.0 - 2026-09-05

### Changed

- `NewMailpit` accepts `MailpitConfig` directly instead of option functions.

### Removed

- `MailpitOption` and `WithMailpitConfig`.

## 0.2.0 - 2026-09-02

### Removed

- Package-level Mailpit default host, port, and `DefaultMailpitConfig`. Applications supply `MailpitConfig`.

### Changed

- `NewMailpit` requires a valid `MailpitConfig` through `WithMailpitConfig`.

## 0.1.0 - 2026-09-01

### Added

- Typed `MailpitConfig` with defaults and validation.
- Functional options for constructing Mailpit clients.
- Shared transactional and marketing send APIs, payload types, and retry helpers.
- Mailpit SMTP client for development environments.

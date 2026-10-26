# Changelog

All notable changes to the standalone Andurel email module are documented here.

## Unreleased

## 0.5.0 - 2026-10-26

### Added

- `SMTPConfig` and `NewSMTP`, a production SMTP sender with STARTTLS,
  implicit TLS, plaintext local relays, PLAIN authentication, and typed
  temporary/permanent errors.
- `SMTP` and `Mailpit` deliver transactional attachments as base64-encoded
  `multipart/mixed` parts, using `Content-Disposition: attachment` or `inline`.

### Fixed

- Messages no longer write a `Bcc` header; Bcc recipients travel only in the
  SMTP envelope.
- Mail headers now include `Date` and `Message-ID`, use a random multipart
  boundary, and RFC 2047 encode non-ASCII subjects and display names.
- Mail headers reject invalid addresses and embedded newlines, preventing
  header injection.
- Message bodies are quoted-printable encoded so long or non-ASCII content
  stays within SMTP line limits.
- SMTP cancellation now interrupts an in-progress conversation, and TLS
  certificate verification failures map to `PermanentError` instead of being
  retried.
- Mailpit honours the send context.
### Changed

- Bump `golang.org/x/net` to v0.60.0 for GO-2026-6617, GO-2026-6612,
  GO-2026-6611, GO-2026-6610, and GO-2026-6603.

## 0.4.0 - 2026-10-26

### Added

- `Transformer` (`ToHTML` / `ToText`) and `Compose`, which renders a template
  into `TransactionalData` for a single recipient.

## 0.3.3 - 2026-09-17

### Changed

- Minimum supported Go version is now 1.27.1.

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

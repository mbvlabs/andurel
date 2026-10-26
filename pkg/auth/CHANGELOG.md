# Changelog

All notable changes to the standalone Andurel auth module are documented here.

## Unreleased

### Changed

- Bump `golang.org/x/crypto` to v0.56.0 for GO-2026-6355 and GO-2026-6354.

## 0.1.0 - 2026-10-26

### Added

- Initial standalone auth module with argon2id password hashing and pepper
  rotation (`HashPassword`, `VerifyPassword`, `VerifyPasswordAny`, `Peppers`).
- Verification codes, secure tokens, and HMAC-SHA256 token hashing
  (`GenerateCode`, `GenerateToken`, `HashToken`, `VerifyToken`).

# Changelog

All notable changes to the standalone Andurel auth module are documented here.

## Unreleased

### Added

- Initial standalone auth module with argon2id password hashing and pepper
  rotation (`HashPassword`, `VerifyPassword`, `VerifyPasswordAny`, `Peppers`).
- Verification codes, secure tokens, and HMAC-SHA256 token hashing
  (`GenerateCode`, `GenerateToken`, `HashToken`, `VerifyToken`).

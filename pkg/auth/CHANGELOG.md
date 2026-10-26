# Changelog

All notable changes to the standalone Andurel auth module are documented here.

## Unreleased

## 0.2.1 - 2026-10-26

### Changed

- Raise the minimum Go version from 1.27.1 to 1.27.3, the release that CI and
  `govulncheck` use, so consumers pick up the standard-library security fixes.

## 0.2.0 - 2026-10-26

### Added

- `DummyVerify` runs a full argon2id verification against a fixed package hash
  and discards the result, so the unknown-user login path costs the same as a
  real verification.
- PHC-format password hashes: `HashPassword` emits
  `$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>` with RawStd base64.

### Changed

- `VerifyPassword` and `VerifyPasswordAny` accept both PHC and legacy
  `base64hash:base64salt` hashes, and verify PHC hashes with the parameters
  parsed from the string.
- `VerifyPasswordAny` reports `needsRehash` for legacy hashes, for PHC hashes
  whose argon2 parameters differ from the current ones, and when a previous
  pepper matched.
- Bump `golang.org/x/crypto` to v0.56.0 for GO-2026-6355 and GO-2026-6354.

## 0.1.0 - 2026-10-26

### Added

- Initial standalone auth module with argon2id password hashing and pepper
  rotation (`HashPassword`, `VerifyPassword`, `VerifyPasswordAny`, `Peppers`).
- Verification codes, secure tokens, and HMAC-SHA256 token hashing
  (`GenerateCode`, `GenerateToken`, `HashToken`, `VerifyToken`).

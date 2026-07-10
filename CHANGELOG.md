# Changelog

All notable changes to Andurel are documented in this file. Andurel follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) beginning with v1.0.0.

## Unreleased

### Added

- A frozen public Go API baseline and automated compatibility checks.
- Stable CLI discovery, structured output, projection, error-code, and exit-code contracts.
- Lock schema 1 with verified per-platform tool downloads.
- Transactional framework-owned file upgrades with dry-run, conflict detection, and rollback.
- Read-only doctor diagnostics, safe DDL parser failures, and deterministic formatter checks.
- Linux and macOS release archives for amd64 and arm64, SBOMs, signed checksums, provenance, and native installation smoke tests.

### Changed

- Release automation now validates the exact tag commit and keeps the release private until every artifact gate succeeds.
- Stable installation documentation uses `@latest` or an explicit v1 tag.

### Security

- Executable tool downloads require a matching SHA-256 digest for the current platform.
- Dependency and standard-library versions include fixes for the reachable advisories identified during v1 hardening.
- Release checksum and SBOM manifests use keyless Sigstore signatures.

## v1.0.0-rc.3 - 2026-07-09

Third public release candidate. Release-candidate projects are not supported upgrade sources for v1.

## v1.0.0-rc.2 - 2026-07-08

Second public release candidate. Release-candidate projects are not supported upgrade sources for v1.

## v1.0.0-rc.1 - 2026-07-06

First public release candidate. Release-candidate projects are not supported upgrade sources for v1.

[Unreleased]: https://github.com/mbvlabs/andurel/compare/v1.0.0-rc.3...HEAD
[v1.0.0-rc.3]: https://github.com/mbvlabs/andurel/releases/tag/v1.0.0-rc.3
[v1.0.0-rc.2]: https://github.com/mbvlabs/andurel/releases/tag/v1.0.0-rc.2
[v1.0.0-rc.1]: https://github.com/mbvlabs/andurel/releases/tag/v1.0.0-rc.1

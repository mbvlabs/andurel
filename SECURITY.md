# Security policy

## Supported versions

After v1.0.0 is released, the latest v1 minor release receives security fixes. The final v1 release candidate is supported only until v1.0.0 is available. Older release candidates and nightly builds are unsupported.

| Version | Supported |
| --- | --- |
| Latest v1.x | Yes |
| v1.0.0-rc.4 before v1.0.0 | Yes |
| RC.1 through RC.3 | No |
| Nightly builds | No |

## Reporting a vulnerability

Report suspected vulnerabilities through [GitHub private vulnerability reporting](https://github.com/mbvlabs/andurel/security/advisories/new). Include the affected version, operating system and architecture, reproduction steps, impact, and any suggested mitigation.

Do not open a public issue, discussion, or pull request for an unpatched vulnerability. If private vulnerability reporting is unavailable, contact the maintainers through the private contact methods on the repository owner's GitHub profile and state that the message concerns an Andurel security report.

The maintainers will acknowledge a complete report as soon as practical, investigate it privately, and coordinate disclosure after a fix is available. Release timing depends on severity, exploitability, and the safety of the remediation.

## Release integrity

Official releases include SHA-256 checksums, per-archive SBOMs, keyless signatures, and GitHub artifact attestations. Follow [the release verification guide](docs/release-verification.md) before installing a downloaded archive.

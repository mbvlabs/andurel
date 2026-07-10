# Phase 4 RC fixture sources

These focused fixtures were extracted from the `postgresql.golden` scaffold at the exact tags below:

- `v1.0.0-rc.1` at `efe84d1bd001c9fd862712e0bdc685e5b7e64419`
- `v1.0.0-rc.2` at `5d782882d82adb1b2326132d4e92a5911de3659a`
- `v1.0.0-rc.3` at `70deaaa1f36e7a35de776d470d832db0d31a0d6a`

Each pristine fixture contains only `andurel.lock`, `router/router.go`, `models/user.go`, and `cmd/app/main.go`. The scaffold golden normalizes the framework version as `<ANDUREL_VERSION>`; the lock fixture rehydrates that field with the exact source tag. No other golden content is regenerated.

Each release also has a `variants.json` manifest defining pristine, independently edited, unknown, missing, duplicated, and ambiguous outcomes used by the Phase 4 regression matrix.

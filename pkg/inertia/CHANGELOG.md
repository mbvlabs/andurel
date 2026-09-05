# Changelog

All notable changes to the standalone Andurel Inertia module are documented here.

## 0.3.0 - 2026-09-05

### Added

- `DefaultConfig`, `DefaultSSRConfig`, and `DefaultManagedConfig`.

### Changed

- `New` initializes protocol and SSR settings from those defaults.
- Managed SSR treats a zero runtime major as the package default and rejects an explicitly negative value.

### Removed

- `WithConfig`. Applications set protocol fields through `WithContainerID`, `WithProtocolDebug`, and `WithSSRFailFast`.

## 0.2.0 - 2026-09-02

### Changed

- `New` no longer applies package defaults. Applications must supply values through options.

### Removed

- `DefaultConfig`, `DefaultSSRConfig`, `DefaultManagedConfig`, and related default constants.

## 0.1.1 - 2026-09-01

### Changed

- Reformatted source with golines; no functional changes.

## 0.1.0 - 2026-08-25

### Added

- Echo-native Inertia v3 protocol renderer.
- Vite asset integration and support for application-owned compiled templ documents.
- Response-level SSR renderer seam.
- Bounded external HTTP SSR renderer in the root package.
- Optional managed JavaScript SSR runtime with `Renderer.Start` and `Renderer.Shutdown` lifecycle methods.
- Functional options for protocol, assets, Vite, and SSR configuration.

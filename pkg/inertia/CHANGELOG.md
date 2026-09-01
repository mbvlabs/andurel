# Changelog

All notable changes to the standalone Andurel Inertia module are documented here.

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

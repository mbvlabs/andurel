# Changelog

All notable changes to the standalone Andurel routing module are documented here.

## 0.2.0 - 2026-09-07

### Added

- `InertiaRoute` route setup option (default false) and `IsInertia()` accessor to opt
  routes into generated Inertia TypeScript helpers via `resources/js/routes.ts`.

### Changed

- Minimum supported Go version is now 1.27.0.
- Route constructors accept optional `RouteSetupOption` arguments after path, name,
  and prefix.

## 0.1.1 - 2026-09-01

### Changed

- Reformatted source with golines; no functional changes.

## 0.1.0 - 2026-08-20

### Added

- Initial standalone routing module with typed route URL generation for simple, UUID, serial, string, and struct-parameter routes.
- Query parameter and JavaScript expression helpers.

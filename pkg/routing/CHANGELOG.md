# Changelog

All notable changes to the standalone Andurel routing module are documented here.

## Unreleased

## 0.4.0 - 2026-09-27

### Added

- `HostName` type and `HostPrimary` constant for named virtual hosts.
- `Host(name HostName)` route setup option (defaults to `HostPrimary`).
- `Host()` accessor on route types.
- Boot-time hostname registry (`ConfigureHosts`, `HostSpec`, `LookupHost`,
  `Hosts`, `HostBaseURL`, `HostSpec.Origins`) so `FullURL()` resolves the
  route's host origin without a base argument.

### Changed

- `FullURL` no longer takes a `base` argument. The origin comes from the
  boot-time host registry using the route's stored `HostName`.

## 0.3.0 - 2026-09-17

### Changed

- Minimum supported Go version is now 1.27.1.
- UUID route helpers now use the standard library `uuid` package instead of
  `github.com/google/uuid`.

## 0.2.1 - 2026-09-09

### Changed

- Minimum supported Go version is now 1.26.0.

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

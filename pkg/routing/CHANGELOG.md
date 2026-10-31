# Changelog

All notable changes to the standalone Andurel routing module are documented here.

## Unreleased

## 0.9.0 - 2026-10-31

### BREAKING

- A route whose path is `/` matches only `/`, on every host. Before, a bare `/`
  was a subtree pattern and caught every path, so `admin.example.com/assets/app.js`
  returned the dashboard. A catch-all must now be written `/*`. Before/after for an
  SPA fallback:

  ```go
  // before: caught every path
  routing.NewSimpleRoute("/", "spa", "")

  // after: explicit catch-all
  routing.NewSimpleRoute("/*", "spa", "")
  ```

  `Pattern()` for a `/` route is `/{$}` with or without `Exact()`.

### Added

- `WildcardParam` constant (`"path"`), the name a trailing `*` segment becomes
  in an `http.ServeMux` pattern (`{path...}`). Read the wildcard with
  `anduin.PathValue(ctx, routing.WildcardParam)`.

### Changed

- `Exact()` is a no-op so existing route files still compile. It can be deleted.
- `Route.Name()` (and the typed route variants) return the declared name without
  the URL prefix. Before, `NewSimpleRoute("/new", "new_session", "/users").Name()`
  returned `users.new_session`; now it returns `new_session`, matching what the
  CLI route tooling shows. Nothing in andurel depended on the prefixed form.
- `NewSimpleRoute("/", "index", "/users")` no longer logs "path has trailing
  slash" at boot. It is the documented way to declare `/users` and is accepted
  quietly.

## 0.8.0 - 2026-10-30

### Added

- `Route.Prefix()` returns a route's subtree root by removing a trailing `/*`
  wildcard (`/assets/dist/*` becomes `/assets/dist`), and
  `Route.InSubtree(path)` reports whether a request path is that prefix or a
  descendant. Asset, CSRF, and CORS skips can use them instead of
  `strings.TrimSuffix(routes.ViteBuild.Pattern(), "/*")` and a hand-built
  `HasPrefix` check, so a later pattern change cannot silently widen the skip.

## 0.7.0 - 2026-10-28

### Added

- `(*Hosts).Resolve(authority)` maps an HTTP authority (`Request.Host`:
  hostname, `hostname:port`, or a bracketed IPv6 literal) to its configured
  `HostName`. Matching is case-insensitive, aliases resolve to their host, and
  a bare configured hostname claims every port (not only `:80`/`:443`).
  Unknown authorities and a nil `*Hosts` return `("", false)`.

### Changed

- `NewHosts` rejects hostname and alias collisions case-insensitively:
  `Admin.example` and `admin.example` on two `HostName`s is now a boot error.

## 0.6.0 - 2026-10-27

### Added

- `TypeScriptRoute` route setup option for routes that need generated
  TypeScript helpers (`resources/js/routes.ts`), including non-Inertia JSON
  endpoints and asset URLs.
- `Exact()` route setup option and `Pattern()` on every route type. `Path()`
  keeps returning the URL (for example `/`), while `Pattern()` returns the
  `http.ServeMux` pattern (`/{$}` for an exact root route), so exact routes no
  longer register a catch-all prefix.

### BREAKING

- The route interfaces (`SimpleRoute`, `UUIDIDRoute`, `ParamRoute` and the
  rest) gain `Pattern()`. Code that implements them outside this module must
  add the method.

### Changed

- `InertiaRoute` is now a deprecated alias for `TypeScriptRoute`; existing
  route files keep working.

## 0.5.1 - 2026-10-26

### Changed

- Raise the minimum Go version from 1.27.1 to 1.27.3, the release that CI and
  `govulncheck` use, so consumers pick up the standard-library security fixes.

## 0.5.0 - 2026-10-26

### BREAKING

- Replaced the package-level `ConfigureHosts` registry and its `LookupHost`,
  `Hosts`, and `HostBaseURL` accessors with an explicit `*Hosts` value from
  `NewHosts(map[HostName]HostSpec)` and `(*Hosts).Lookup`, `All`, and `BaseURL`.
- `FullURL` now takes the `*Hosts` value to resolve the route's origin.

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
- `ConfigureHosts` rejects hostname/alias collisions across distinct
  `HostName`s.
- `HostBaseURL` returns an empty string for unregistered names instead of
  falling back to the primary origin.

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

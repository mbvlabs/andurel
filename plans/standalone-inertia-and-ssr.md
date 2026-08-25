# Standalone Inertia v3 package and SSR runtime

## Status

Implementation plan for Andurel v2. V2 does not preserve generated V1
`internal/inertia` APIs or perform an obsolete-file migration.

## Goals

- Ship the complete Echo-native Inertia v3 implementation as the independent
  `github.com/mbvlabs/andurel/pkg/inertia` module.
- Give the package sane defaults for Vite, protocol, and disabled SSR behavior;
  keep the compiled initial document application-owned.
- Expose functional options for application configuration and customization.
- Keep generated controllers and the router on direct `pkg/inertia` imports.
- Put HTTP and managed SSR in the root `inertia` package rather than a nested Go
  package.
- Keep `pkg/inertia` independent of Fx and environment-loading libraries.
- Expose renderer lifecycle methods for managed SSR.
- Keep ENV declarations and Inertia composition in `config/inertia.go`.
- Keep the JavaScript package manager separate from the SSR executable.

## Non-goals

- V1 source compatibility or generated-file upgrade migration.
- A generic `net/http` adapter.
- An Fx dependency in the standalone module.
- Reimplementing official Inertia browser clients.
- Making SSR apply automatically to every page response.

## Ownership

### Standalone module

```text
pkg/inertia/
├── assets.go
├── config.go
├── constants.go
├── errors.go
├── flash.go
├── middleware.go
├── page.go
├── props.go
├── props_resolver.go
├── redirect.go
├── renderer.go
├── request.go
├── root.go
├── ssr.go
├── ssr_config.go
├── ssr_http.go
└── ssr_managed.go
```

The module owns:

- the Inertia v3 protocol and page object;
- Echo middleware, redirects, and responses;
- prop resolution and shared props;
- Vite development and production asset tags;
- bounded HTTP SSR communication;
- optional managed JavaScript process ownership;
- SSR health, timeout, response-size, and lifecycle behavior.

The package does not read application environment variables, import Fx, or know
about generated application packages.

### Generated application

```text
application/metadata.go
config/inertia.go
views/root.templ
resources/js/app.*
resources/js/ssr.*
```

`config/inertia.go` owns ENV declarations, renderer composition, and lifecycle
wiring. `application/metadata.go` holds shared, non-sensitive application
identity. `views/root.templ` is the application-owned compiled initial document.

There is no generated `internal/inertia` package.

## Constructor and options

The package constructor applies defaults and then functional options:

```go
renderer, err := inertia.New(options...)
```

Defaults include:

- container ID `app`;
- development environment and conventional Vite URL;
- `resources/js/app.ts` entry point;
- SSR disabled;
- loopback SSR URL;
- bounded request and startup timeouts;
- bounded SSR response size;
- Node runtime and minimum supported major version;
- CSR fallback rather than SSR fail-fast.

Options cover stable protocol configuration, asset filesystem, project name,
environment, build URL, Vite URL, entry point, custom root, shared/flash
providers, custom SSR renderer, SSR mode, URL, executable, bundle, timeouts,
response limits, and fail-fast behavior.

Generated applications supply the application-owned `views.Root` with
`inertia.WithRoot`.

## SSR

`SSRRenderer` remains the protocol seam:

```go
type SSRRenderer interface {
    Render(context.Context, Page) (*SSRResponse, error)
}
```

The root package provides `HTTPRenderer` and `ManagedRuntime`. Normal generated
applications configure those indirectly through renderer options.

`Renderer` owns any configured managed runtime and exposes:

```go
Start(context.Context) error
Shutdown(context.Context) error
```

These methods are no-ops for disabled, external, and custom SSR renderers.
Managed mode validates the executable, runtime version, loopback URL, bundle,
and limits; starts exactly one process; waits for bounded health readiness; and
shuts it down with bounded fallback behavior. External mode never owns the
external process lifecycle.

SSR remains opt-in per initial response through `WithSSR()`. JSON visits never
contact SSR. Failure falls back to CSR unless fail-fast is configured.

## Generated Fx wiring

`cmd/app/main.go` supplies `application.Metadata` and includes
`config.InertiaModule`. The private constructor in `config/inertia.go` translates
`config.Inertia`, supplies `views.Root`, and registers the renderer lifecycle.

## Verification

- Run `gofmt` and `go fix` in the root and standalone module.
- Run `go vet ./...` in both modules; do not use `go build` or `go test` under
  the repository instructions.
- Scaffold Vue, React, and Svelte applications with `GOWORK=off` module
  resolution and vet the generated Go code using a local module replacement
  until `pkg/inertia/v0.1.0` is tagged.
- Verify disabled, external, and managed SSR configuration.
- Verify bounded HTTP responses, health readiness, shutdown, and CSR fallback.
- Update scaffold goldens, public contracts, documentation, and CI standalone
  module vetting.

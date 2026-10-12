# Generated files and upgrade behavior

Andurel distinguishes framework-owned files from application files. Application code includes controllers, models, views, jobs, routes, migrations, configuration, application entrypoints, and generated `internal/` (including `internal/runtime`). Normal upgrades do not rewrite those trees unless a command explicitly documents a narrower generated declaration boundary. `andurel upgrade` also pins required `github.com/mbvlabs/andurel/pkg/*` modules in `go.mod` to the versions verified with that CLI. It does not downgrade a newer pin. `andurel packages update` still chases the latest published package versions independently of a framework upgrade.

V2 is not an upgrade from V1. Projects on `1-5-stable` continue to use V1's upgrade behavior. V2 projects on `master` are created fresh with `andurel new`; there is no automated V1-to-V2 migration path.

Factory synchronization is one such narrow boundary. It regenerates the Andurel-owned factory types, `Build<Name>`, `Create<Name>`, `Create<Name>s`, and generated `WithX` option functions. Custom helpers whose names do not collide with generated declarations are preserved.

The historical Inertia root embedding migration is another narrow boundary. It moves `views/root.go.html` to `assets/inertia/root.go.html` and replaces only the exact legacy `inertia.Init("views/root.go.html")` call in `cmd/app/main.go`. The upgrade stops without writing if that call is not present. Current V2 scaffolds own a compiled templ document at `views/root.templ`; the composition root supplies `views.Root` with `inertia.WithRoot`.

## V1-era migrations

The V1 upgrade notes for session-cookie decode recovery (`v1.5.4`) and Fx-managed Inertia renderer injection (`v1.5.6`) are not part of the V2 CLI. V1 projects reach them by upgrading on `1-5-stable` with a V1 `andurel` binary. The V2 CLI refuses to upgrade a project across a major version.

## Inertia renderer

The current V2 scaffold imports the complete implementation directly from the
standalone `github.com/mbvlabs/andurel/pkg/inertia` module. Generated
`config/inertia.go` owns ENV-backed settings, renderer composition, and Fx
lifecycle wiring; there is no generated `internal/inertia` package. Controllers remain application-owned and
are never rewritten.

## Standalone telemetry package

V2 scaffolds construct `*telemetry.Telemetry` from `github.com/mbvlabs/andurel/pkg/telemetry`. Generated `config/telemetry.go` still loads env values. Composition roots unpack those fields into positional `telemetry.New` plus `With*` options. Fx injects the provider into HTTP (`otelhttp`) and Postgres (`otelpgx`) instrumentation. Application code uses `telemetry.Error`, `Start`, and `Set` instead of a copied `telemetry/` package or `slog.*Context`. Controllers call `telemetry.From(etx, name)` then pass the returned `context.Context` to `Set` / `Error`. Services call `telemetry.Start(ctx, name, …)`.

Every V2 release, starting with `v2.0.0-alpha`, ships this layout, so `andurel upgrade` emits no telemetry manual action.

## Inertia Vite FullURL migration

Starting with `v2.0.0-alpha.3`, Inertia production tags need an absolute Vite build URL so a secondary-host page loads the entry from the primary origin. Pass `routes.ViteBuild.FullURL()` into the renderer. `pkg/inertia` emits `crossorigin` on those tags. Generated `internal/runtime` is application-owned, so `andurel upgrade` does not rewrite `NewInertia`.

When an Inertia project upgrades from a version before `v2.0.0-alpha.3` to `v2.0.0-alpha.3` or later, `andurel upgrade` emits a version-gated manual action:

1. In `NewInertia` (or the inertia renderer provider), pass `routes.ViteBuild.FullURL()` instead of `Path()`.
2. Run `gofmt`, `go fix ./...`, and `go vet ./...` after reconciling application-owned code.

Projects created at `v2.0.0-alpha.3` or later do not receive the note.

## Provider adapters move under `internal/`

External provider adapters (API clients for payment providers, email providers, Discord, and similar third-party services) now belong in a package under `internal/` instead of a top-level `clients/` directory. `internal/` holds application-owned support code that is not a domain model, service, controller, or view. Name each package for its job, for example `internal/clients/` for a group of API clients or `internal/payment/` for one payment adapter. One home for this code keeps the top level of the app smaller. Fx wiring for adapters stays in `internal/runtime`. Andurel never generated `clients/`, so `andurel upgrade` does not move hand-written adapters. To follow the new convention in an existing app:

1. Move each `clients/<x>` package under `internal/`. The smallest move is `git mv clients internal/clients`.
2. Rewrite imports to match, for example `<module>/clients/<x>` → `<module>/internal/clients/<x>`.
3. Run `gofmt`, `go fix ./...`, `go vet ./...`, and `go build ./...`.

## Planning and preview

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, tool metadata changes, unified diffs, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final lock version last. Any replacement or validation failure rolls every changed file and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. The automated upgrader requires an explicit `schemaVersion` and does not accept release-candidate locks. Projects created with v1.0.0-rc.2 or v1.0.0-rc.3 must use the [RC-to-v1 manual upgrade guide](upgrade-rc-base-scaffold-prompt.md).

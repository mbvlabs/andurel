# Generated files and upgrade behavior

Every file `andurel new` writes belongs to the application: `cmd/`, `config/`, `domains/`, `handlers/`, `middleware/`, `internal/runtime/`, `queue/`, `email/`, `resources/`, `migrations/`, and the rest. Framework behavior lives in the standalone `github.com/mbvlabs/andurel/pkg/*` modules, which the application imports like any other library.

`andurel upgrade` therefore never rewrites application source. A real upgrade:

1. pins the `github.com/mbvlabs/andurel/pkg/*` requires in `go.mod` to the versions verified with the target CLI, without downgrading a newer pin;
2. synchronizes the tool versions in `andurel.toml`;
3. writes the target version to `andurel.toml` and refreshes the per-platform tool hashes in `andurel.lock`;
4. reports a manual action for every application-owned change the target release needs.

`andurel packages update` chases the latest published package versions independently of a framework upgrade.

## Project metadata: andurel.toml vs andurel.lock

The two files have different owners and different jobs:

| File | Owner | Contents |
| --- | --- | --- |
| `andurel.toml` | human-editable project manifest | `schemaVersion`, the framework `version`, `[project]` (`name`, Inertia adapter, JavaScript package manager and SSR runtime), `[database]` (`engine`, `nullType`), and `[tools]` (`tool = "version"` pins) |
| `andurel.lock` | CLI-owned digest lock | one `[[hashes]]` entry per tool and platform: `tool`, `version`, `platform`, `sha256` |

Read `andurel.toml` for the framework version, adapter, package manager, and
tool pins. `andurel.lock` carries no project metadata of its own; it records the
digests `andurel tool sync` verifies when it downloads a pinned binary. The CLI
writes both, and `andurel inspect project` reads both. Edit `andurel.toml` by
hand only when you know the target release supports the change; let
`andurel tool set-version` and `andurel upgrade` own the lock; hand-editing a
digest makes the next download fail verification. Commands that read the
adapter (`andurel sync routes`, `andurel sync payloads`) and commands that pin
tools (`andurel tool set-version`) read and write `andurel.toml`, not
`andurel.lock`.

## Supported upgrade paths

`andurel upgrade` accepts projects created at `v2.0.0-alpha.4` or later. `v2.0.0-alpha.4` replaced Echo and Fx with the Go-native `domains`/`handlers`/`internal/runtime` layout, so older V2 projects must be recreated with `andurel new` and ported by hand.

V2 is not an upgrade from V1. Projects on `1-5-stable` continue to use V1's upgrade behavior with a V1 `andurel` binary. The V2 CLI refuses to upgrade a project across a major version.

Release readiness runs `TestUpgradeToHEAD` (`just test-upgrade-e2e`), which scaffolds with each supported released CLI, upgrades with the release candidate, applies every reported manual action, and requires the result to build and vet cleanly.

## Manual actions

A manual action is emitted only when the upgrade crosses the release that introduced the change. Each action has a stable `id`, a title, and instructions with before/after code. Read them from `andurel upgrade --json` or the human report, apply them, then run `gofmt -w .`, `go fix ./...`, and `go vet ./...`.

Upgrading from `v2.0.0-alpha.4` to `v2.0.0-alpha.5` or later reports:

| ID | Change |
| --- | --- |
| `package-api-v2.0.0-alpha.5` | Composition-root changes for the alpha.5 `pkg/*` APIs. |
| `storage-storagetest-v2.0.0-alpha.5` | Test databases come from `pkg/storage/storagetest`. |
| `routing-hosts-v2.0.0-alpha.5` | `routing.NewHosts` replaces the package-level host registry; `FullURL` takes the registry. |
| `session-revocation-v2.0.0-alpha.5` | `users.session_version`, checked by `middleware.AuthOnly(users)`; admin status is read from the database. |
| `email-smtp-v2.0.0-alpha.5` | `EMAIL_PROVIDER=smtp` and the production Mailpit guard. |
| `validation-validate-v2.0.0-alpha.5` | `validation.Validate` is gone; call the value's own `Validate` method. |

Upgrading from `v2.0.0-alpha.5` to `v2.0.0-beta.1` reports no manual actions.

Upgrading from any earlier supported release to `v2.0.0-beta.2` or later reports:

| ID | Change |
| --- | --- |
| `email-sender-v2.0.0-beta.2` | The default sender drops the `HOST_PRIMARY` port, and `NewMail` rejects an invalid sender at startup. |
| `vite-ssr-bundle-v2.0.0-beta.2` | Inertia projects: `vite.config.ts` bundles SSR dependencies into the production build only. |
| `root-title-ssr-v2.0.0-beta.2` | Inertia projects: `resources/root.templ` renders the fallback `<title>` only for client-rendered pages. |

The other beta.2 scaffold changes (exact home route and themed 404, flash
messages, compression, `andurel build` output) are defaults for new
applications; existing applications can copy them from a fresh `andurel new`.

Upgrading from any earlier supported release to `v2.0.0-beta.3` or later reports:

| ID | Change |
| --- | --- |
| `cors-host-origins-v2.0.0-beta.3` | Global CORS trusts only `CORS_ALLOWED_ORIGINS`; Vite build files answer with wildcard, uncredentialed CORS. Security-relevant for apps with more than one configured host. |
| `csrf-asset-skip-v2.0.0-beta.3` | Safe requests for Vite build files no longer receive the CSRF cookie. |
| `method-not-allowed-page-v2.0.0-beta.3` | A method mismatch renders the themed `Errors/NotFound` page with status 405 and the `Allow` header. |
| `request-host-middleware-v2.0.0-beta.3` | Advisory: replace hand-written host matching with `anduin.RequestHost(ctx)`, and use `middleware.DenyFraming` for host-scoped headers. |

The other beta.3 scaffold changes (host-aware docs, the `andurel verify`
skip/warning output and cross-host guidance) are defaults for new
applications; existing applications can copy them from a fresh `andurel new`.

Upgrading from any earlier supported release to `v2.0.0-beta.4` or later reports
the beta.3 actions above plus:

| ID | Change |
| --- | --- |
| `public-home-v2.0.0-beta.4` | Register the welcome page without `middleware.AuthOnly`; protected routes keep their auth middleware and anonymous `GET /` returns 200. |
| `configcheck-v2.0.0-beta.4` | Add the read-only `internal/configcheck` helper `andurel doctor` runs for runtime configuration and database connectivity. |
| `runtime-readiness-v2.0.0-beta.4` | Add `GET /api/ready`: 200 `{"status":"ready"}` or 503 `{"status":"unready","reason":"..."}` for an unavailable database or required SSR sidecar. `/api/health` stays liveness. |
| `inertia-ssr-default-v2.0.0-beta.4` | Inertia projects: production server-renders every page with `inertia.WithSSRDefault(cfg.App.IsProduction())`; `.CSR()` opts a page out and the themed `Errors/InternalError` fallback stays client-rendered. Uses `pkg/inertia v0.13.0`. |
| `routes-host-scope-v2.0.0-beta.4` | Inertia projects: regenerate `routes.ts` so full-URL helpers throw before configuration, configure browser hosts at boot, and wrap each SSR render with `withRouteHosts`. |

Beta.4 pins `pkg/inertia v0.13.0`. A development install can still point at a
local framework checkout with `just local-replace <app>`; remove these
replacements before verifying a published-module installation.

## Provider adapters under `internal/`

External provider adapters (API clients for payment providers, email providers, Discord, and similar third-party services) belong in a package under `internal/`, named for its job: `internal/clients/` for a group of API clients or `internal/payment/` for one payment adapter. Wiring for adapters stays in `internal/runtime`. Andurel never generated a top-level `clients/` directory, so `andurel upgrade` does not move hand-written adapters.

## Planning and preview

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, tool metadata changes, unified diffs, manual actions, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final manifest version last. Any replacement or validation failure rolls every changed file, `andurel.toml`, and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. The upgrader requires an explicit `schemaVersion` in `andurel.toml`.

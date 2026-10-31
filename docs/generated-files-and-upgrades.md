# Generated files and upgrade behavior

Every file `andurel new` writes belongs to the application: `cmd/`, `config/`, `domains/`, `handlers/`, `middleware/`, `app/`, `payloads/`, `queue/`, `email/`, `resources/`, `migrations/`, and the rest. Framework behavior lives in the standalone `github.com/mbvlabs/andurel/pkg/*` modules, which the application imports like any other library.

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

Release readiness runs `TestUpgradeToHEAD` (`just test-upgrade-e2e`), which scaffolds with the previous released CLI, upgrades with the release candidate, and requires the result to build and vet cleanly.

## Manual actions

Upgrade notes only exist between proper releases. Alpha and beta releases carry no manual actions: scaffold changes between pre-releases are defaults for new applications, and an existing application copies them from a fresh `andurel new` if it wants them.

A manual action is emitted only when the upgrade crosses the release that introduced the change. Each action has a stable `id`, a title, and instructions with before/after code. Read them from `andurel upgrade --json` or the human report, apply them, then run `gofmt -w .`, `go fix ./...`, and `go vet ./...`.

## Provider adapters

External provider adapters (API clients for payment providers, email providers, Discord, and similar third-party services) belong in a top-level package named for its job: `clients/` for a group of API clients or `payment/` for one payment adapter. Generated apps have no top-level `internal/`. Wiring for adapters stays in `app/`.

## Planning and preview

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, tool metadata changes, unified diffs, manual actions, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final manifest version last. Any replacement or validation failure rolls every changed file, `andurel.toml`, and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. The upgrader requires an explicit `schemaVersion` in `andurel.toml`.

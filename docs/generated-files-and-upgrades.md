# Generated files and upgrade behavior

Every file `andurel new` writes belongs to the application: `cmd/`, `config/`, `domains/`, `handlers/`, `middleware/`, `internal/runtime/`, `queue/`, `email/`, `resources/`, `migrations/`, and the rest. Framework behavior lives in the standalone `github.com/mbvlabs/andurel/pkg/*` modules, which the application imports like any other library.

`andurel upgrade` therefore never rewrites application source. A real upgrade:

1. pins the `github.com/mbvlabs/andurel/pkg/*` requires in `go.mod` to the versions verified with the target CLI, without downgrading a newer pin;
2. synchronizes the tool versions in `andurel.toml`;
3. writes the target version to `andurel.lock`;
4. reports a manual action for every application-owned change the target release needs.

`andurel packages update` chases the latest published package versions independently of a framework upgrade.

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

## Provider adapters under `internal/`

External provider adapters (API clients for payment providers, email providers, Discord, and similar third-party services) belong in a package under `internal/`, named for its job: `internal/clients/` for a group of API clients or `internal/payment/` for one payment adapter. Wiring for adapters stays in `internal/runtime`. Andurel never generated a top-level `clients/` directory, so `andurel upgrade` does not move hand-written adapters.

## Planning and preview

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, tool metadata changes, unified diffs, manual actions, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final lock version last. Any replacement or validation failure rolls every changed file and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. The upgrader requires an explicit `schemaVersion` in `andurel.lock`.

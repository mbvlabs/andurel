# V2 stability policy

This policy says what an application built on Andurel V2 can rely on, starting with `v2.0.0-beta.1`.

## What changes between releases

An Andurel application has three parts that change on different schedules:

| Part | Owner | How it changes |
| --- | --- | --- |
| Generated application code (`cmd/`, `config/`, `domains/`, `handlers/`, `middleware/`, `internal/runtime/`, `queue/`, `resources/`, …) | the application | Never rewritten after `andurel new`. A release that needs a change ships it as a manual action. |
| `github.com/mbvlabs/andurel/pkg/*` modules | Andurel | Independent semantic versions. The CLI pins the versions it was verified with. |
| The `andurel` CLI | Andurel | Command names, flags, and structured output follow the rules below. |

## Pre-releases

Each pre-release is upgradeable from the previous one, and every supported upgrade path is tested in release readiness: the old CLI scaffolds an app, the new CLI upgrades it, every manual action is applied, and the result must build and vet.

**Alpha** (`v2.0.0-alpha.N`). Anything can change. `andurel upgrade` accepts projects from `v2.0.0-alpha.4` on.

**Beta** (`v2.0.0-beta.N`). The architecture is fixed: `net/http` through `pkg/anduin`, explicit construction in `internal/runtime`, the `domains`/`handlers` layout, Inertia for the UI, and the agent-facing CLI contract. A beta can still change application code or a `pkg/*` API, but only to fix a bug, a security problem, or a mistake found while building on the beta. Every such change ships with:

- a manual action with a stable ID and before/after code, emitted by `andurel upgrade`;
- an entry in the release notes;
- a `### BREAKING` changelog entry in each affected `pkg/*` module.

**Stable** (`v2.0.0` and later). Within `v2.x`, a minor or patch release does not require changes to application code. New scaffold improvements are opt-in: they ship as optional manual actions and as the default for new apps. A change that would break application code waits for `v3`.

## `pkg/*` modules

The modules stay on `v0.x`, and each has its own changelog.

- A **patch** release (`v0.x.Y`) does not change exported APIs. It fixes bugs, raises dependency or Go versions, or adds documentation.
- A **minor** release (`v0.X.0`) can add APIs. During the beta, it can also remove or change them, under a `### BREAKING` changelog heading.
- From `v2.0.0` on, the APIs that generated application code uses do not break within `v2.x`. If one must break, the old API stays in place under a `Deprecated:` doc comment until the next Andurel major version.

Applications should take package updates through `andurel upgrade`, which pins the verified set. `andurel packages update` moves to the latest published versions without that verification.

## CLI contract

Agents depend on the CLI, so it is versioned like an API:

- Command names, flag names, and the shape of `--json` and `--agent` output keep working within `v2.x`. Fields can be added, never removed or renamed. A removed command or flag stays as a hidden alias for at least one minor release.
- Manual action IDs never change once released.
- `andurel verify` rules can become stricter only in a minor release, and the release notes list each new rule.
- The patterns from `andurel patterns show` and the bundled skills describe the current scaffold. They can change in any release.

## Toolchain

V2 requires Go 1.27.3 or later. Raising the minimum Go version is allowed in a minor release of the CLI or a patch release of a `pkg/*` module, and is listed in the release notes.

## Not covered

- Projects created before `v2.0.0-alpha.4`. Recreate them with `andurel new`.
- V1 projects. They stay on the `1-5-stable` branch with a V1 CLI.
- Unexported identifiers, `internal/` packages inside `pkg/*`, and test helpers outside `pkg/storage/storagetest`.

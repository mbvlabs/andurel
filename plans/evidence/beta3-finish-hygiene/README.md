# F40/F06 hygiene and doctor acceptance evidence

Owner: `fix/beta3-finish-hygiene` (separate from `scripts/beta3-fix-acceptance.sh`).
Script: `scripts/acceptance/beta3-hygiene-doctor.sh`.

This lane closes the two remaining matrix gaps:

- **F40**: lockfile/frozen-install and repository hygiene are now proven on a
  brand-new generated app, not only documented.
- **F06**: the live doctor negatives (missing, malformed, exported-only, dead
  database) are now automated with secret-safe assertions.

It also covers scoped/changed/content verify behavior where it is cheap and
useful. It is **development proof only**: the generated app points at this
checkout's `pkg/*` modules with temporary `go.mod` replaces because the
candidate `pkg/inertia` pin is unpublished. It never publishes, tags, pushes, or
touches another worktree.

## Run

Standalone:

```bash
scripts/acceptance/beta3-hygiene-doctor.sh
```

Require every probe (blocked probes exit nonzero):

```bash
scripts/acceptance/beta3-hygiene-doctor.sh --require-all
```

Choose an evidence directory (the default is
`plans/evidence/beta3-finish-hygiene/<candidate-sha12>-hygiene-doctor`):

```bash
scripts/acceptance/beta3-hygiene-doctor.sh \
  --evidence-dir plans/evidence/beta3-finish-hygiene/<sha>-hygiene-doctor
```

Prerequisites: Go, node, pnpm, git, jq, and network access for the pnpm registry
and pinned tool downloads. Missing prerequisites are reported as `blocked`, never
as a pass.

## Coordinator integration

The integration branch now invokes this script as probe `19-hygiene-doctor` in
development mode. Release mode explicitly reports this companion blocked rather
than treating development replacements as published-module proof. It can also
be run as a separate required stage:

```bash
"$ROOT/scripts/acceptance/beta3-hygiene-doctor.sh" \
  --evidence-dir "$EVIDENCE/hygiene-doctor" \
  --require-all
```

The script exits `0` only when every probe passed, `1` on a failed probe (or any
blocked probe with `--require-all`), and records `probes.tsv`, `probes.md`,
`environment.txt`, `candidate-commit.txt`, `candidate-status.txt`,
`candidate-pins.txt`, `acceptance-state.txt`, and per-command
stdout/stderr/status files.

## Latest result

Evidence: `plans/evidence/beta3-finish-hygiene/0d6cb0ba6573-hygiene-doctor/`

```
passed=9 blocked=0 failed=0
acceptance: full
```

| Probe | What it proves |
| --- | --- |
| `01-new-react-pnpm-harness` | `andurel new --harness opencode` creates the default React/pnpm app, installs the selected skill files only, ships no lockfile, and documents the frozen command; a bare non-interactive `skill install` is a usage error. |
| `02-local-replacements` | The candidate CLI app builds against local `pkg/*` replaces; `go vet` passes; setup is incomplete before sync/install and names `andurel tool sync` + `pnpm install`. |
| `03-lockfile-frozen-install` | `pnpm install` generates `pnpm-lock.yaml`; the lockfile is tracked/unignored; `pnpm install --frozen-lockfile` succeeds without rewriting it; a drifted `package.json` fails with `ERR_PNPM_OUTDATED_LOCKFILE`; restoring the manifest restores frozen success. |
| `04-setup-positive` | Real `andurel tool sync` installs every pinned binary; `verify --only setup` passes; doctor reports tool versions and Project setup as `pass` while Runtime config still fails, so setup green does not mask an unverified runtime. |
| `05-doctor-missing-env` | No `.env` and no exported values: doctor exits nonzero, Runtime config is `fail`/blocking, and the diagnostic names `HOST_PRIMARY`. |
| `06-doctor-malformed-env` | A malformed `SESSION_KEY` exits nonzero, names the key, and never echoes the rejected secret. |
| `07-doctor-export-only-unreachable-db` | Exported-only configuration is valid; a dead database (`127.0.0.1:1`) is `database is unreachable` with a connection-refused reason and no password. |
| `08-hygiene-ignored-outputs-secrets` | `.env`, `node_modules`, `bin`, `dist`, `assets/dist`, `tmp`, and the SQLite file stay ignored/untracked; `.env.example`, `andurel.toml`, `andurel.lock`, `pnpm-lock.yaml`, and the selected skill files are tracked; the `.env` marker never reaches the index. |
| `09-verify-scoped-changed-content` | Content is `not_applicable` without a manifest, `ok` when valid, and fails with `content-link`; `verify --only gofmt --changed` records the changed file/package scope and fails exactly the unformatted file. |

## Suggested matrix rows

| ID | Status | Evidence |
| --- | --- | --- |
| F40 | implemented, live dev evidence | `scripts/acceptance/beta3-hygiene-doctor.sh` probes 01/03/04/08; `0d6cb0ba6573-hygiene-doctor` |
| F06 | implemented, live negative evidence | `scripts/acceptance/beta3-hygiene-doctor.sh` probes 05/06/07; `0d6cb0ba6573-hygiene-doctor` |

## Historical unrelated blocker found by `go test ./...`

Resolved on the integration branch by `5e9f8221`; the coordinator's root suite
passes. The following records the worker's original baseline result:

`go test ./...` still fails at `cli/patterns`
(`TestCompositionPatternMatchesScaffoldRuntime`): the test expects
`handlers.NewAPIHandler(db, newReadiness(cfg, db))` while the scaffold template
and composition pattern use `newReadiness(db, renderer)`. This lane did not
change readiness or patterns; the failure is pre-existing on the integrated
branch and needs the owning lane to decide whether the test expectation or the
template is authoritative. All other packages passed, including `cli`, `layout`,
`golden`, and `skills`.

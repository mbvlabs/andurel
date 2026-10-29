# Beta.3 complete-fix integration state (recovery checkpoint)

Status: **integration implemented and locally validated; full acceptance is
PARTIAL.** No publication, tag, or push has happened. Site adoption commits exist
in lane F but are not merged into the site's `port` branch. Independent live
fresh-scaffold/site/release reviews are still required.

Integration worktree: `/home/mbv/work/worktrees/andurel-beta3-fixes/G`, branch
`fix/beta3-complete-integrated`.

## Resumed finishing work (2026-10-29)

The integration work survived the previous session outside `master`. The
untracked specification in the original master checkout still says implementation
has not started; that describes the original handoff, not this integration branch.

The coordinator resumed at `0d6cb0ba` and committed:

- `5e9f8221`: candidate beta.4 release notes and the stale readiness-wiring test
  assertion corrected to the renderer-backed implementation.
- `396baf95`: recovered implementation lint fixes; root suite and lint pass.

Current independent workers use DeepSeek V4.1 Flash through OpenCode Go, with
one writer in each checkout under `/home/mbv/work/worktrees/andurel-beta3-fixes/`:

| Checkout | Task |
| --- | --- |
| `finish-live` | Durable live fresh-app/SSR/build acceptance and gate 17 wiring. |
| `finish-hygiene` | F40 frozen-install/tracking policy and F06 doctor negatives. |
| `finish-db-tests` | F20 actual uncached generated database test execution. |
| `finish-upgrade` | Exercise beta.4 manual actions, not merely the old beta.3 target. |
| `F` | Independent site analytics/content/SSR/readiness adoption verification. |

Hygiene/doctor commits `5f77033c` and `3566f960` have been integrated as
`05b317f2` and `147457b5`; all nine probes also passed an independent coordinator
rerun. The upgrade worker's `0907b12a` is integrated as `0ede6573`: the E2E now
targets beta.4 and applies all five new actions to alpha.4 through beta.3 source
apps. The coordinator is rerunning that expanded suite with `GOWORK=off` for
the CLI build as well.

The database-test worker's commits are integrated (`6d832a65`, `6afcf31f`,
`a6ddf1f6`, `a47d98af`); the coordinator independently reran its gate: 15 named
database tests passed uncached and verify reported 28 passes, 0 failures/skips.
It is wired as `20-database-tests` under `--full`.

The site worker completed `d7457d6` and `bfaf158` on lane F, with 45 passing
development checks. Coordinator isolation/cleanup refinements are committed as
`22a62ff`, and its independent rerun also passed all 45 checks. The main harness accepts `--site-dir` and
can run the site gate rather than unconditionally blocking it. It still labels
the development replacements accurately and blocks release mode for that
companion.

Only the live fresh-scaffold worker is still in progress. See
`plans/evidence/beta3-complete-fix/finish-2026-10-29/checks.md` for coordinator
validation. No publication or push is authorized.

## Commits on this branch

- D `83bc452a`, `8854f66d` → `fdfc9166`, `454458e3`
- C `0fa1ccf1` → `e12e018b`
- E `a17516a8` → `2c04fcdc`
- A `b14d9ee2` → `d76640ba`
- B `a3b6cf11` → `75aa00bb`; approved B follow-up `d829b3d5` → `994ecb91`
- Integration commits after `994ecb91`: ports registration, route-usage alias
  and IIFE support, doctor runtime-check failure, production SSR default and
  unready readiness schema, beta.4 upgrade manual actions, build publication
  recovery, port ownership proof, candidate module proxy isolation, acceptance
  harness (see `git log --oneline 994ecb91..HEAD`).

## Integrated behavior

- **Ports (E)**: `andurel ports launch` is registered in the real command tree
  with catalog/projection metadata. A built-CLI test proves JSON publication,
  ready-file cleanup, clean SIGTERM shutdown, and nonzero missing-child
  failure. Linux also proves the listening socket belongs to the launcher child
  process group before the URL is published; other platforms fail closed rather
  than publish an unverified URL. Direct fixed-address sidecar operation remains
  available there.
- **Readiness (D/A)**: `GET /api/health` stays liveness. `GET /api/ready`
  answers `200 {"status":"ready"}` or
  `503 {"status":"unready","reason":"<reason>"}`; a required SSR sidecar that
  is down reports `{"status":"unready","reason":"unavailable"}`, and a database
  failure reports `database_unavailable`. Readiness uses
  `Renderer.Health`, so it recovers without a restart.
- **SSR policy (D/A)**: the scaffold renderer uses
  `inertia.WithSSRDefault(cfg.App.IsProduction())`; development keeps per-page
  `.SSR()`, production pages opt out with `.CSR()`. `cmd/app` only warns at boot
  when fail-fast is on. Error mappings stay client-rendered so a failed SSR
  render cannot recurse.
- **Route usage (B/D/G)**: the rule now resolves `tsconfig.json`
  `compilerOptions.paths` aliases (the scaffold's `@/*`), immediately invoked
  function/arrow expressions, and bare top-level calls; unresolved aliases,
  deferred bodies, and `.path()` stay allowed.
- **Setup/doctor (A/B/G)**: doctor and verify share one installed-tool
  predicate, verify uses the package manager recorded in `andurel.lock`, and a
  generated `internal/configcheck` that fails to build is a doctor failure, not
  a skip. A missing helper or missing Go toolchain stays a blocked skip. The
  `new` and `doctor` catalog metadata carry the shared Quick Start and runtime
  checks.
- **Build (E/G)**: interrupted output publication is repaired before a new
  isolated build publishes: a backup without its final binary is restored, a
  stale backup is removed, and temporary outputs are removed. Recovered items
  are reported in `recovered`.
- **Upgrade (G)**: beta.4 manual actions cover public home registration, the
  configcheck helper, readiness wiring, production SSR default with themed
  error fallback, and routes bootstrap/`withRouteHosts` adoption. They are
  candidate-gated on `v2.0.0-beta.4`.
- **Docs/goldens**: README/AGENTS templates, patterns, docs, CLI catalog, and
  all slim/full scaffolds are regenerated. Full goldens pass with the candidate
  module proxy.

## Candidate pin and release gate (BLOCKED)

`layout/versions.Inertia` is the candidate pin `v0.13.0`. The tag
`pkg/inertia/v0.13.0` does **not** exist; publication is not authorized. The
`pkg/inertia/CHANGELOG.md` section says "candidate, requires publication".
Fresh scaffolds reference v0.13.0 and require the release gate before a
release-shaped install can succeed. Development integration uses temporary
`go.mod` replaces (`just local-replace`) or the golden candidate module proxy;
neither is release-shaped proof.

### Module-cache contamination (reported and quarantined)

Earlier golden runs (before cache isolation) downloaded the fake local
`pkg/inertia@v0.13.0` into the global module cache. The affected entries were
moved to:
`/home/mbv/.cache/andurel-retest/quarantine-gomodcache-inertia-v0.13.0/`
(extracted module + `cache/download/.../@v/v0.13.0.{info,mod,zip,ziphash,lock}`).
No broad cache removal was done. `internal/goldentest.CandidateGoEnv` now uses a
dedicated temporary `GOMODCACHE`, `GOCACHE`, and `GOTMPDIR` under the proxy
directory, sets `GOENV=off`, and returns an error instead of silently falling
back to ambient env. The proxy and caches are removed by
`CleanupCandidateModuleProxy` after golden tests. Verify global cache with:
`test ! -e "$(go env GOMODCACHE)/github.com/mbvlabs/andurel/pkg/inertia@v0.13.0"`.

## Acceptance harness

`scripts/beta3-fix-acceptance.sh` creates a fresh `mktemp` tree, builds the
candidate CLI and `scripts/acceptance/healthchild`, runs 18 probes, records
command/stdout/stderr/status, and writes `probes.tsv`, `probes.md`,
`environment.txt`, `candidate-commit.txt`, `candidate-pins.txt`, and
`acceptance-state.txt`. It traps and removes only its own processes/scratch.

Commands:

```bash
# Development integration (dev-app uses temporary pkg replaces; not release proof)
scripts/beta3-fix-acceptance.sh --mode dev

# Release-shaped attempt; reports the unpublished-pin gate blocked
scripts/beta3-fix-acceptance.sh --mode release

# Full acceptance: any blocked probe exits nonzero
scripts/beta3-fix-acceptance.sh --mode dev --require-all
```

Latest dev run: `passed=16 blocked=2 failed=0` (evidence
`plans/evidence/beta3-complete-fix/653de5af95f5-dev/`; rerun after each new
commit). Blocked probes are **not** passes: they are
`17-live-runtime-gates` (live DB/frontend/SSR/build flow is documented but not
automated in this harness revision) and `18-site-gate` (separate site worktree).
A run with any blocked probe writes `acceptance: partial`.

## Honest 19-ID matrix

| ID | Status | Evidence / remaining gap |
| --- | --- | --- |
| F01 | implemented, live gate blocked | public home template + generated runtime tests; live anonymous `/` requires DB/frontend assets (harness gate 17). |
| F04 | implemented, dev evidence | harness 02/03/07/08 pass; untouched scaffold reports setup, not drift; documented setup reaches green verify only after tools/deps/DB. |
| F06 | implemented, partial live | runtime config check fails on build error, blocks missing/invalid values and dead DB, distinguishes prerelease/network in unit tests; live doctor negatives not automated. |
| F07 | implemented | inspect pages/process/urls/packages tests and goldens; harness 01 CLI surface passes. |
| F10 | implemented | `commands --check`, truthful JSON/jq registrations, harness 01 passes. |
| F18 | implemented | scoped stages/rules/changed tests in `cli/verify_scope_test.go`; not part of harness probes. |
| F20 | implemented | per-package test accounting, harness 09 passes; real uncached DB tests not run. |
| F21 | implemented framework, site blocked | content invariants harness 08 passes in a dev scaffold; site content fixtures (gate 18) not run. |
| F22 | implemented, release-gated | `WithSSRDefault`/`CSR` wired; generated full scaffold vet passes with candidate proxy; live production SSR gate blocked; needs published v0.13.0. |
| F25 | implemented static/runtime, live gate blocked | alias/IIFE rule tests pass; D request-scoped host scope wired; live two-host SSR not automated. |
| F32 | implemented, live gate blocked | readiness schema/template tests; live sidecar loss/recovery gate blocked. |
| F35 | site-only blocked | no framework change; site analytics not verified here. |
| F36 | implemented | metadata/version consistency + harness 01; release notes for beta.4 not written. |
| F39 | implemented, live gate blocked | build isolation/recovery + port ownership tests; concurrent output-dir builds and real sidecar wrapper not run in harness. |
| F40 | implemented docs/policy | README/AGENTS lockfile/skill policy; hygiene/frozen-install probe not automated. |
| N1 | implemented | harness 01 unknown inspect is nonzero `usage_error`; bare inspect/help succeeds. |
| N2 | implemented | harness 06 `inspect config --jq .keys` works. |
| N3 | implemented | harness 05 migration creation without `.env`/DB works. |
| N4 | implemented, live gate blocked | generated `cmd/ssr` lifecycle test passes; occupancy/post-start crash re-run against generated binary is part of blocked live gate. |

## Remaining work for independent verifier

1. Implement/run the live gate behind `17-live-runtime-gates` (disposable
   PostgreSQL, tool sync, pnpm install/build, migrations, production app +
   `cmd/ssr`): public `/`, SSR markup, readiness loss/recovery, themed 500,
   build isolation, wrapper failure.
2. Run `--mode release` after `pkg/inertia/v0.13.0` and `v2.0.0-beta.4` are
   published; until then the release gate is blocked, not passed.
3. Run the separate andurel-site adoption checks (F35/F32/F21/F25) in the site
   worktree.
4. Add F40 hygiene and F06 live negative probes to the harness.
5. Final matrix in `plans/evidence/beta3-complete-fix/<commit>/matrix.md`.

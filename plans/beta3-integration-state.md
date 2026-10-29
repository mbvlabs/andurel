# Beta.3 complete-fix integration state (recovery checkpoint)

Status: **development acceptance PASS; release acceptance BLOCKED on
publication.** The final clean-checkout development suite passed all 20 probes,
including independent live scaffold, site, hygiene/doctor and uncached DB tests.
No publication, tag, push, or merge to master/port has happened.

Tested framework: `dcec50dc85f86b555f84887bcfe0bf7ee566ed55`.
Tested site: `3ffd3b4ead6f7a6ace5c85e0575c369ae85d5b59`.
Final 19-ID matrix and exact commands:
`plans/evidence/beta3-complete-fix/dcec50dc-dev/matrix.md`.

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

All five independent workers completed using DeepSeek V4.1 Flash through
OpenCode Go, with one writer in each checkout under
`/home/mbv/work/worktrees/andurel-beta3-fixes/`:

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
apps. The coordinator reran that expanded suite with `GOWORK=off` for the CLI
build as well: all five source versions passed.

The database-test worker's commits are integrated (`6d832a65`, `6afcf31f`,
`a6ddf1f6`, `a47d98af`); the coordinator independently reran its gate: 15 named
database tests passed uncached and verify reported 28 passes, 0 failures/skips.
It is wired as `20-database-tests` under `--full`.

The site worker completed `d7457d6` and `bfaf158` on lane F, with 45 passing
development checks. Coordinator isolation/cleanup refinements are committed as
`22a62ff`, followed by release-mode support in `3ffd3b4`. Its independent rerun
and final full-suite run both passed all 45 checks. The main harness accepts
`--site-dir` and runs the site gate rather than unconditionally blocking it.

The live worker's `f2e08332`/`9136e483` were integrated as
`9600144e`/`9f9bfde9`. Coordinator follow-up `dcec50dc` adds plain verification
after setup, explicit CSR fallback, healthy-sidecar/render-failure coverage,
actual execution of both isolated output pairs without source assets, safe
container identity cleanup, and publication guards. The final live gate passed
78 checks. See `plans/evidence/beta3-complete-fix/finish-2026-10-29/checks.md`
for the chronological validation history. No workers remain active.

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
candidate CLI and `scripts/acceptance/healthchild`, runs 20 development probes, records
command/stdout/stderr/status, and writes `probes.tsv`, `probes.md`,
`environment.txt`, `candidate-commit.txt`, `candidate-pins.txt`, and
`acceptance-state.txt`. It traps and removes only its own processes/scratch.

Commands:

```bash
# Development integration (dev-app uses temporary pkg replaces; not release proof)
scripts/beta3-fix-acceptance.sh --mode dev

# Full development suite, including the independent site; any blocked probe fails
scripts/beta3-fix-acceptance.sh --mode dev --full --require-all \
  --site-dir /home/mbv/work/worktrees/andurel-beta3-fixes/F

# Published-pin suite: currently stops at the publication prerequisite
scripts/beta3-fix-acceptance.sh --mode release --full --require-all \
  --site-dir /home/mbv/work/worktrees/andurel-beta3-fixes/F
```

Latest full dev run: **`passed=20 blocked=0 failed=0`**, exit 0. Framework and
site checkouts were both clean. Live scaffold: 78 passing checks; site: 45;
hygiene/doctor: 9. Fifteen named DB tests actually ran uncached, and verify
reported 28 passes, zero failures/skips and no cached DB packages.

Raw final evidence is retained at
`/home/mbv/.cache/andurel-retest/final-dcec50dc/`; the durable repository matrix
links each finding to implementation, development and negative regression
evidence. Earlier evidence directories remain historical snapshots, not current
acceptance claims.

Release mode first verifies the candidate module tag, before any generated-app
check can fetch an unpublished module. Its current strict result is exit 1,
`passed=0 blocked=1 failed=0`. After publication it uses published pins without
local replacements, `GOWORK=off`, public proxy/checksum settings and a fresh
private module cache. Companions support that mode, but it has not passed.

## All 19 findings and remaining release work

See `plans/evidence/beta3-complete-fix/dcec50dc-dev/matrix.md`: all 19 rows have
implementation and passing development regression evidence; every release-shaped
column is explicitly blocked on publication, not silently treated as passed.

1. Follow `docs/releasing.md` for the standalone module changelog/tag/pins PR
   sequence. Do not publish or push without authorization.
2. Merge site adoption through the site's normal workflow after compatible
   package publication; its `port` branch is untouched so far.
3. Repeat the strict release-shaped suite above after publication, without
   workspace or local replacements. Preserve failure evidence and fix any
   published-module-only issue before claiming release acceptance.
4. Complete master CI/readiness and the beta.4 CLI release under the repository
   release policy. Candidate notes already exist in
   `docs/release-notes/v2.0.0-beta.4.md`.

The original all-release definition of done is **not complete** until these
gates pass. Development completion is not authorization to tag, merge or push.

# Beta.3 fixes — final development acceptance

**Development suite: PASS. Release acceptance: BLOCKED on publication.**

- Framework: `dcec50dc85f86b555f84887bcfe0bf7ee566ed55`, clean checkout,
  branch `fix/beta3-complete-integrated` (worktree `G`).
- Site: `3ffd3b4ead6f7a6ace5c85e0575c369ae85d5b59`, clean checkout,
  branch `fix/beta3-complete-F` (worktree `F`). Site implementation and regression
  evidence include `d00594a`, `0722df2`, `d7457d6`, and `bfaf158`; coordinator
  isolation/release-mode refinements are `22a62ff` and `3ffd3b4`.
- Candidate CLI version injected through the existing linker contract:
  `v2.0.0-beta.4`. This identifies an **unpublished acceptance build**, not a
  published release. The SHA above identifies the tested source exactly.
- Date: 2026-10-29. Go 1.27.3, Node 26.12.1, pnpm 12.9.1, Docker 29.8.2.
- Full harness: **20 passed, 0 blocked, 0 failed**, exit 0 with `--require-all`.
  Live scaffold: **78 checks passed**; site: **45 checks passed**;
  hygiene/doctor: **9 probes passed**.
- Actual DB tests: **15 named uncached passes**; verify: 4 tested packages,
  28 passes, 0 failures/skips, 5 passing subtests, 17 no-test-file packages.

## Exact reproduction

From the integration worktree:

```bash
TMPDIR=/tmp/opencode GOTMPDIR=/tmp/opencode GOWORK=off \
  scripts/beta3-fix-acceptance.sh --mode dev --full --require-all \
  --site-dir /home/mbv/work/worktrees/andurel-beta3-fixes/F \
  --evidence-dir /tmp/opencode/beta3-coordinator/full-dcec50dc
```

Raw evidence is retained outside the repository at
`/home/mbv/.cache/andurel-retest/final-dcec50dc/` (also at the original output
directory). It contains per-command stdout/stderr/status, command/cwd records,
candidate/tool metadata, build manifests, HTTP responses with scrubbed cookies,
and nested live/site/hygiene/database reports. No generated `.env` is retained.
Large raw artifacts are linked rather than duplicated in Git.

References below are relative to that raw evidence directory:

- `probes.md` / `probes.tsv`: all 20 outer probes.
- `17-live-runtime-gates/live/`: 78 live checks, real SSR, failures, and builds.
- `site/`: independent application checks and live runtime evidence.
- `hygiene-doctor/`: lockfile/tracking/setup and config negatives.
- `database-tests/`: actual named pass events and per-package verify accounting.
- `cleanup-final.txt`: known owned containers removed; all five scratch trees
  absent; no processes reference their paths. The first diagnostic attempt
  matched its own checker command; the final check excludes checker ancestry.

## All 19 findings

Every row below has passing development regression evidence. **Every release
column remains blocked**, not passed: no published `pkg/inertia v0.13.0` exists.
Implementation commits are the integrated commits, not worker-only hashes.

| ID | Implementation | Development evidence and negative regression | Release |
| --- | --- | --- | --- |
| F01 | `d76640ba`, `653de5af` | Probe 17: anonymous home 200, SSR Welcome markup, login/session preserved. Probe 20 runs named DB auth/session tests; generated runtime tests retain explicit AuthOnly protection. | blocked |
| F04 | `d76640ba`, `75aa00bb`, `653de5af` | Probes 02/03/19: consistent setup instructions, missing setup reported separately; probe 17 proves plain verify passes after tool sync/frontend install without source edits. | blocked |
| F06 | `d76640ba`, `4865319a`, `05b317f2` | Probes 04/12/19: missing/malformed/exported-only config, dead DB, secret-safe diagnostics, helper failures and prerelease/network distinctions. Live app uses the valid configured DB. | blocked |
| F07 | `e12e018b` | Probe 01 plus inspect-view/generated-app tests: pages/process/URLs/package responsibilities; unknown config/associations stay unresolved rather than fabricated. | blocked |
| F10 | `e12e018b` | Probe 01 and executable capability tests: JSON/jq registrations and contradictory-metadata failures; unsupported DB output modes remain usage errors. | blocked |
| F18 | `75aa00bb`, `994ecb91`, `ca5f0711` | Probe 19 checks real changed-file scope. Root scope tests cover selection, invalid names/base, no Git, untracked/deleted/renamed files and widening. | blocked |
| F20 | `75aa00bb`, `994ecb91`, `6d832a65` | Probes 09/10/20: pass/fail/skip accounting and opt-in behavior; 15 real named DB passes uncached, 28 verify passes, no hidden DB skips or cached DB packages. | blocked |
| F21 | `75aa00bb`, `ca5f0711`; site commits above | Probes 08/19 break catalog targets, duplicate slugs and links. Probe 18 runs the site's actual catalog invariants and mutations; absent content config is explicitly N/A. | blocked |
| F22 | `fdfc9166`, `d76640ba`, `653de5af`; site `0722df2` | Probe 17: production renderer-wide SSR, real rendered markup, explicit CSR fallback. Module tests cover per-page overrides and JSON visits; all adapter full goldens pass. | blocked |
| F25 | `454458e3`, `589fc43e`, `ca5f0711`, `0d6cb0ba`; site `0722df2` | Probe 11 exercises eager/aliased route misuse. Probe 18 renders 12 concurrent distinct-host requests through the real SSR bundle without host leakage; safe `.path()`/deferred cases remain supported. | blocked |
| F32 | `fdfc9166`, `d76640ba`, `653de5af`; site `0722df2` | Probes 17/18: SSR loss -> readiness 503 + themed 500, restart -> recovery; DB loss/recovery; healthy `/health` with failing render still gives themed 500; explicit CSR fallback separately tested. | blocked |
| F35 | Site `d00594a`, `d7457d6` | Probe 18: configurable custom analytics in production HTML; excluded error/admin pages omit it; disabled/default/invalid/custom configuration and exclusions have regression tests. Scaffold remains analytics-free. | blocked |
| F36 | `e12e018b`, `653de5af`, `5e9f8221`, `0ede6573`, `dcec50dc` | Metadata/version checks, candidate version injection, accurate migration prerequisites, candidate release notes. Expanded beta.4 upgrade E2E applies all five new actions to all five supported source versions. | blocked |
| F39 | `2c04fcdc`, `b9359602`, `17e2c03d`, `dcec50dc` | Probes 13–17: atomic publication/recovery, exact listener ownership, distinct held leases; two concurrent isolated builds preserve source outputs and both app/SSR pairs run from empty runtime dirs without source assets/node_modules. | blocked |
| F40 | `d76640ba`, `05b317f2` | Probe 19: selected harness only, committed skill/lockfiles, frozen reinstall unchanged, drift rejected, secrets and outputs ignored/untracked. | blocked |
| N1 | `e12e018b` | Probe 01: unknown inspect subcommand exits nonzero with typed usage error; bare help remains successful. | blocked |
| N2 | `e12e018b` | Probe 06: actual `inspect config --jq .keys` succeeds. | blocked |
| N3 | `e12e018b` | Probe 05: migration creation succeeds without `.env` or a reachable DB; configuration-free file creation retains validation errors. | blocked |
| N4 | `2c04fcdc` | Probe 17: compiled generated SSR exits 1 on occupied listener with EADDRINUSE and after killing its JS child. Compiled-template tests additionally cover missing bundle, invalid config and clean signal shutdown. | blocked |

## Repository gates

All completed successfully against the integration source:

- Root `gofmt -l`, `go fix -diff ./...`, `go vet ./...`,
  `go test ./... -count=1 -timeout 20m`.
- `ANDUREL_GOLDEN_FULL=1 GOWORK=off go test ./golden -count=1 -timeout 45m`
  (113.465s final rerun). Uses the isolated development candidate proxy,
  **not published-module proof**; the global fake v0.13.0 cache remains absent.
- Targeted CLI ports/build/route race tests (8.710s final rerun) and standalone
  `pkg/inertia` formatting/fix/vet/race tests. Race tests enable CGO with Zig CC.
- `just lint`: 0 issues. ShellCheck and Bash syntax checks for acceptance scripts.
- Expanded local-package upgrade E2E: alpha.4, alpha.5, beta.1, beta.2, beta.3
  -> beta.4, all five manual actions applied, then tidy/build/vet (64.701s).
- `just check-release-notes v2.0.0-beta.4`.

Exact commands and earlier failed/corrected checks are recorded in
`../finish-2026-10-29/checks.md`. An earlier site runner was edited while it was
executing and failed parsing; it was rerun from a stable file and passed, then
passed again in this full clean-checkout suite. It is not counted as a pass.

## Release gate and remaining authorization

The same command with `--mode release` and `--require-all` exits **1** with
`00-release-prerequisite: blocked`, before creating an app or downloading the
unpublished module. Evidence:
`/home/mbv/.cache/andurel-retest/final-dcec50dc/release-dcec50dc/`.

Follow `docs/releasing.md`: publish the changed standalone module through the
required changelog/pins PR sequence, merge the site adoption through its normal
workflow, then repeat the release-shaped suite with published pins, no workspace
or local replacements, and a private fresh module cache. The main harness and
companions support release mode, but it has **not passed**. Master CI/readiness
and the beta.4 CLI release are separate release gates.

No merge to master/port, tag, publication or push was performed. Do not mark the
original all-release definition of done complete on the strength of this
development result alone.

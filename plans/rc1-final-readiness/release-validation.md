# rc.1 framework release-shaped validation (post-pin)

Status: **READY FOR INDEPENDENT REVIEW. LOCAL RELEASE-SHAPED GATES PASS,
EXCEPT THE INTENTIONALLY NON-PASSING `just check-fix` WRAPPER ON THIS DIRTY
WORKTREE. CI ARTIFACT PREFLIGHT IS PENDING ON A PUSHED FINAL MASTER SHA. THIS
IS NOT A RELEASE GO/NO-GO.** No commit, tag, push, publication or deployment
was performed.

Prepared: 2026-11-02. This record extends `pin-integration.md`; it does not
repeat the package publication review or the independently accepted runtime
reviews. It is local framework validation only, not independent review, not
site T09/T07 and not a release verdict.

## Identity and environment

```text
Framework:  /home/mbv/work/andurel-rc1-work
Branch:     fix/rc1-final-readiness
HEAD:       c6c724fa8beb0b563fd4966d3d7748af70ad6f46
Go:         go1.27.3 linux/amd64
GOPROXY:    https://proxy.golang.org,direct
GOSUMDB:    sum.golang.org
GOWORK:     off for every just recipe and affected standalone module run
Lint:       golangci-lint 2.14.0 (pinned)
Vuln:       govulncheck v1.3.0 from /tmp/opencode/rc1-tools
Race:       CGO_ENABLED=1, CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc', Zig 0.14.1
```

The worktree is intentionally dirty from accepted rc.1 work. Before the
test-only change described below it had 19 tracked files modified (1123
insertions, 139 deletions); after adding `layout/upgrade/upgrade_e2e_test.go`
it has 20 tracked files modified (1252 insertions, 157 deletions). The original
19-file change set is unchanged apart from this test file. The three modified
`testdata/golden/new/postgresql-inertia-*/go.mod.golden` files are the only
expected golden changes. The pre-change baseline is
`evidence/release-validation-baseline.txt`; the current post-test state is
`evidence/release-validation-final-state-post-candidate-test.txt`. No
production service, database or container was used. `test-upgrade-e2e` used
only its own disposable `t.TempDir()` app and public module downloads, with no
Docker or database.

### Source identity and pending narsilc #857 integration

This validation ran on local HEAD `c6c724fa8beb0b563fd4966d3d7748af70ad6f46`
plus the dirty rc.1 changes. `origin/master` has since advanced to
`ee84032c62987cdcfca3d93a183709808681ef23` (`chore(layout): scaffold narsilc
v0.6.4 (#857)`), which is **not merged** into this worktree. The current source
still pins Narsilc tool/module `v0.6.3` and its lock goldens contain the
v0.6.3 hashes; `origin/master` carries the v0.6.4 hashes and later Anduin/
Inertia pin integration is still pending on the normal merge path. Evidence:
`evidence/release-validation-source-identity.txt`.

The added beta.5/beta.7 -> rc.1 E2E cases do not assert a narsilc version and
are therefore not blocked by #857. Their rc.1 manifest and anduin/inertia pin
expectations remain valid for this source. After the parent merges #857 and the
remaining pin work, the E2E matrix and full release gates must be rerun on the
merged tree; this record does not cover `narsilc v0.6.4`. The site is already
on 0.6.4 and was not touched here.

Per instruction, the costly full release golden was **not** rerun here. The
pin worker's release-shaped run remains the evidence:

```text
$ just test-golden-full-release
PASS
ok  github.com/mbvlabs/andurel/v2/golden  125.520s
EXIT: 0
```

Evidence: `evidence/pin-integration-test-golden-full-release.txt`. That pass
predates #857 and covers the **Narsilc v0.6.3 scope only**; it is historical for
the narsilc dimension and must not be reused after the v0.6.4 merge. The only
golden diff in this worktree is the three expected `go.mod.golden` files
recorded in `pin-integration.md`; the v0.6.4 `andurel.lock.golden` hashes are
not present yet.

## Gate results

| Gate | Command | Exit | Result | Evidence |
| --- | --- | --- | --- | --- |
| Published pins | `just check-published-pins` | 0 | 12/12 public pins verified, including anduin `v0.8.0` and inertia `v0.14.1`; kiks remains `v0.6.1` | `evidence/release-validation-check-published-pins.txt` |
| Module metadata | `just check-mod` | 0 | `all modules verified`; `go mod tidy -diff` produced no output | `evidence/release-validation-check-mod.txt` |
| Formatting | `just check-fmt` | 0 | empty output; re-run after the test change | `evidence/release-validation-check-fmt.txt`; `evidence/release-validation-check-fmt-post-candidate.txt` |
| Vet | `just vet` | 0 | root plus all 12 standalone `pkg/*` modules; re-run after the test change | `evidence/release-validation-vet.txt`; `evidence/release-validation-upgrade-candidate-verification.txt` |
| Root go fix | `GOWORK=off go fix -diff ./...` | 0 | empty diff and empty stderr; re-run after the test change | `evidence/release-validation-go-fix-diff.txt`; `evidence/release-validation-upgrade-candidate-verification.txt` |
| `check-fix` wrapper | `just check-fix` | **1** | **Expected failure**, only at its final `git diff --exit-code -- .` because the accepted rc.1 changes are uncommitted. Its go fix stage was clean: `/tmp/go-fix.diff` was 0 bytes. This wrapper is **not** claimed as passed. | `evidence/release-validation-check-fix.txt`; `evidence/release-validation-check-fix-gofix.diff` |
| Pinned lint | `TMPDIR=/tmp/opencode just lint` | 0 | golangci-lint 2.14.0, `0 issues`; re-run after the test change | `evidence/release-validation-lint.txt`; `evidence/release-validation-lint-post-candidate.txt` |
| Pinned vulnerability scan | `PATH=/tmp/opencode/rc1-tools:$PATH just vulncheck` | 0 | govulncheck 1.3.0; root plus all 12 standalone modules, no called vulnerabilities | `evidence/release-validation-vulncheck.txt`; `evidence/release-validation-govulncheck-version.txt` |
| Root race suite | `CGO_ENABLED=1 CC='.../zig cc' TMPDIR=/tmp/opencode just test-race` | 0 | All 36 packages selected by the recipe (every root package except `golden`); 29 `ok` plus `[no test files]`; key times: cli 42.692s, layers 161.553s, layout 19.467s, upgrade 2.902s | `evidence/release-validation-test-race-root.txt` |
| Standalone race: anduin | `GOWORK=off ... go test -mod=readonly -race -count=1 ./...` | 0 | anduin 1.401s, anduintest 1.029s | `evidence/release-validation-test-race-pkg-anduin.txt` |
| Standalone race: inertia | same, in `pkg/inertia` | 0 | 1.641s | `evidence/release-validation-test-race-pkg-inertia.txt` |
| Standalone race: kiks | same, in `pkg/kiks` | 0 | 3.159s | `evidence/release-validation-test-race-pkg-kiks.txt` |
| Standalone metadata/fix | `go mod verify`, `go mod tidy -diff`, `go fix -diff ./...` in affected modules | 0 | `all modules verified`; empty tidy/fix diffs; no `replace` directives | `evidence/release-validation-standalone-modules.txt`; `evidence/release-validation-standalone-go-fix-diff.txt` |
| Upgrade E2E matrix | `GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org just test-upgrade-e2e` | 0 | beta.4 -> beta.5 regression plus beta.5 -> rc.1 and beta.7 -> rc.1 candidate paths; all three pass, candidate manifest/pins/no-replace assertions pass, 13.943s | `evidence/release-validation-test-upgrade-e2e-candidate.txt` (historical single-path run: `evidence/release-validation-test-upgrade-e2e.txt`) |
| Release notes validator | `just check-release-notes v2.0.0-rc.1` | 0 | prints `--release-notes docs/release-notes/v2.0.0-rc.1.md`; notes remain an explicit DRAFT | `evidence/release-validation-check-release-notes.txt` |

`just test` was not run separately. `test-race` and `test` select exactly the
same root package set (`go list ./... | grep -v '/golden$'`, 36 packages), so
the root race run covers the full root test set with the stronger race check.
The recipe comparison is in `evidence/release-validation-root-suite-scope.txt`.

`check-fix` needs a clean committed tree for its final `git diff`; it cannot
legitimately pass on this shared worktree. Committing or hiding the accepted
changes to force it green would be wrong. The repository's CI runs this recipe
on a clean checkout, but that future CI result is not claimed here.

## Published pins and module identity

`just check-published-pins` resolved every framework pin through the public
proxy and checksum database with `GOWORK=off`:

```text
ok   github.com/mbvlabs/andurel/pkg/anduin@v0.8.0
ok   github.com/mbvlabs/andurel/pkg/inertia@v0.14.1
ok   github.com/mbvlabs/andurel/pkg/kiks@v0.6.1
... 9 more pkg/* pins ...
12 published pkg/* pins verified
```

The exact module sums and origin commits for anduin `v0.8.0` and inertia
`v0.14.1` are already recorded in `pin-integration.md` and
`evidence/pin-integration-module-download.json`; this run revalidated their
public availability and did not edit any pin, `go.mod` or `go.sum`.

For the affected standalone modules, `GOWORK=off`, the public proxy and
`sum.golang.org` were explicit. `go list -m all` showed no `replace` entries;
the relevant published requirements were anduin `v0.8.0` for inertia and kiks,
and routing `v0.9.0`/telemetry `v0.2.3` transitively. All three modules passed
`go mod verify` and had empty `go mod tidy -diff` and `go fix -diff` output.
Root `check-mod` passed independently. The `check-fix` failure is only the
dirty-tree `git diff`, not a go fix suggestion.

## Vulnerability scan detail

The pinned scanner was the workflow version, not the ambient v1.8.0:

```text
Go: go1.27.3
Scanner: govulncheck@v1.3.0
DB updated: 2026-10-27
```

The recipe completed root and every `pkg/*` module (anduin, auth, cache,
email, env, inertia, kiks, lifecycle, routing, storage, telemetry,
validation). It reported `No vulnerabilities found` for each module and
`Your code is affected by 0 vulnerabilities` where package/symbol analysis
applied. As before, some required module versions contain advisories that the
scanner does not find called or imported; this result is not a claim that every
transitive module advisory is absent.

## Upgrade E2E detail

Command (the justfile sets `GOWORK=off`; public proxy/checksums were explicit):

```sh
GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  just test-upgrade-e2e
```

### Historical target (pre-change)

The previous matrix had one released path, `v2.0.0-beta.4` ->
`v2.0.0-beta.5`, and passed in 8.930s. That was the **coverage red**: it did
not exercise the spec's beta.5-to-candidate path. It was not a migration
failure. Historical log:
`evidence/release-validation-test-upgrade-e2e.txt`.

### Added candidate paths (test-only)

`layout/upgrade/upgrade_e2e_test.go` now runs three cases under the existing
`TestUpgradeToHEAD` name, so the existing `just test-upgrade-e2e` recipe picks
them up without a justfile change:

- `v2.0.0-beta.4` -> `v2.0.0-beta.5` (existing released regression);
- `v2.0.0-beta.5` -> `v2.0.0-rc.1`;
- `v2.0.0-beta.7` -> `v2.0.0-rc.1`.

The beta.5/beta.7 scaffold source is a published CLI fetched with
`go run github.com/mbvlabs/andurel/v2@<source> new`. The rc.1 target is passed
directly to this checkout's `NewUpgrader`; there is deliberately no
`v2.0.0-rc.1` source CLI, so the fixture does not claim an rc.1 CLI is
published. Candidate cases assert, after the prescribed `go mod tidy`:

- `andurel.toml`/lock framework version is exactly `v2.0.0-rc.1`
  (`layout.ReadLockFile(...).Version`);
- `go.mod` requires `github.com/mbvlabs/andurel/pkg/anduin v0.8.0` and
  `github.com/mbvlabs/andurel/pkg/inertia v0.14.1`;
- `go.mod` has no `replace` directives;
- the upgrade report has no manual actions. This is the documented policy:
  upgrade notes exist only between proper releases, so pre-release-to-
  pre-release upgrades intentionally emit none (`upgrade_notes_test.go`).

Then the test runs `go build ./...` and `go vet ./...` in the disposable app.
These candidate paths reported no tool changes, so the prescribed binary
`tool sync` is a no-op for them; tool metadata is synced by the upgrade plan,
and the existing recipe contract does not install binaries. Candidate cases are
skipped only when `ANDUREL_UPGRADE_E2E_LOCAL_PKG=1`, because that development
mode deliberately adds local package replacements.

Green output:

```text
=== RUN   TestUpgradeToHEAD
=== RUN   TestUpgradeToHEAD/v2.0.0-beta.4-to-v2.0.0-beta.5
=== RUN   TestUpgradeToHEAD/v2.0.0-beta.5-to-v2.0.0-rc.1
=== RUN   TestUpgradeToHEAD/v2.0.0-beta.7-to-v2.0.0-rc.1
--- PASS: TestUpgradeToHEAD (13.94s)
    --- PASS: TestUpgradeToHEAD/v2.0.0-beta.4-to-v2.0.0-beta.5 (4.68s)
    --- PASS: TestUpgradeToHEAD/v2.0.0-beta.5-to-v2.0.0-rc.1 (4.60s)
    --- PASS: TestUpgradeToHEAD/v2.0.0-beta.7-to-v2.0.0-rc.1 (4.66s)
PASS
ok  github.com/mbvlabs/andurel/v2/layout/upgrade  13.943s
EXIT: 0
```

Full log: `evidence/release-validation-test-upgrade-e2e-candidate.txt`;
test-only diff: `evidence/release-validation-upgrade-candidate.diff`. No
migration failure was observed and no blocker remains from this matrix. The
test used only its disposable temporary scaffold and did not mutate the
worktree outside the test file; it is not a site upgrade or T09 proof.

The actual CLI `--json` flow was not duplicated in a separate scaffold test.
The accepted command-surface suite (`cli/upgrade_json_contract_test.go`, plus
the upgrade error-data tests) already asserts that `commands_run` contains only
actually executed subprocesses and that `go mod tidy` / `andurel tool sync` are
reported as warnings and breadcrumbs, not executed follow-up work. A full CLI
run would redo the expensive scaffold without adding domain coverage here, so
this change stays test-only in the existing E2E recipe.

## Final tree integrity

The current `git status --porcelain=v1` has 20 tracked modifications: the
original 19-file change set (1123 insertions / 139 deletions) plus the
test-only `layout/upgrade/upgrade_e2e_test.go` matrix (129 insertions / 18
deletions). No original tracked path changed its own diff. The untracked files
are unchanged apart from this report and its evidence under the
already-untracked `plans/rc1-final-readiness/` directory. Total tracked diff is
1252 insertions / 157 deletions. `git diff --check` exited 0, and no new
`go.sum`, `go.work`, lock or golden changes appeared from the validation runs.
Evidence: `evidence/release-validation-final-state-post-candidate-test.txt`.

No non-test code, golden, pin or manifest edit was made. The only code change
is the test-only candidate matrix in
`layout/upgrade/upgrade_e2e_test.go`. No commit, tag, push, publication,
deployment or production contact was performed.

## Pending CI artifact preflight (exact requirement)

This gate **cannot be run locally and no local run is a substitute**. The
current candidate changes are uncommitted on `fix/rc1-final-readiness`; there
is no final master commit SHA to validate yet.

The exact required event is a successful push-triggered
`Release Readiness` workflow run on `master` for the final candidate commit,
including its reusable `Release artifact preflight` job:

- `.github/workflows/release-readiness.yml` triggers only on `push` to
  `master` (not pull requests, not local commands).
- Its `readiness` job runs the full recipe set, including `check-fix` on a
  clean checkout, `test-race`, release-shaped goldens and
  `test-upgrade-e2e`, then must confirm no tracked-file mutation.
- Its `artifact-preflight` job runs after `readiness` and calls
  `.github/workflows/release-artifact-preflight.yml` for the exact
  `github.sha`. That reusable workflow builds with GoReleaser
  (`v2.17.0`), signs the SBOM checksum manifest, verifies the four-archive
  linux/darwin amd64/arm64 matrix and checksums/SBOMs, confirms packaging did
  not mutate tracked source, uploads the private preflight artifacts, and runs
  the archive smoke matrix.
- `.github/workflows/release.yml` additionally refuses to release unless there
  is a successful `release-readiness.yml` run on `master` for the exact tagged
  commit with `event=push` and `conclusion=success`.

After the owner commits and pushes the final candidate, verify the exact SHA:

```sh
gh run list \
  --workflow release-readiness.yml \
  --branch master \
  --commit <FINAL_MASTER_SHA> \
  --event push \
  --status success \
  --limit 1 \
  --json databaseId,headBranch,headSha,event,conclusion,url
```

Then inspect that run (for example `gh run view <databaseId> --json jobs`) to
confirm the `Release artifact preflight` job succeeded. This record makes no CI
claim: the result is pending until that pushed master SHA exists and the
workflow reports success.

## Remaining owner gates / non-claims

- The intentional worktree changes must be committed by the owner and pushed
  through the normal master/PR policy; this worker did not commit.
- The narsilc `v0.6.4` #857 merge and the remaining pin integration must land
  first. Then refresh the full release gates on the merged tree, including the
  full release golden/lock hashes, this E2E matrix, race, lint, vulnerability
  scan and published-pin checks. The values in this record are prior-source
  evidence.
- The pushed final master SHA must pass the Release Readiness workflow,
  including artifact preflight. Local gate runs above are evidence input only.
- `just check-fix` will be exercised legitimately only on a clean checkout (the
  CI readiness job); the local wrapper failure above is expected and reported,
  not hidden.
- Release notes still carry the explicit DRAFT/not-release-ready banner and a
  pending package table; the validator passing does not finalize their content.
- Site T09, post-publish B7-005 positive proof and T07 independent live/image
  acceptance remain separate gates as defined in `publication-handoff.md`.
- No rc.1 tag, GitHub release, deployment or production action was performed.
  The stop point for this worker is local validation evidence only, not
  eligible-to-tag.

## Evidence index

All new logs are under `plans/rc1-final-readiness/evidence/`:

- `release-validation-baseline.txt` — branch, HEAD, Go/tool versions, go env,
  initial status.
- `release-validation-source-identity.txt` — local HEAD vs `origin/master`
  #857, current vs upstream narsilc/anduin/inertia pins and narsilc lock-golden
  hash difference.
- `release-validation-check-published-pins.txt` — 12/12 public pins.
- `release-validation-check-mod.txt` — root module download/verify/tidy.
- `release-validation-check-fmt.txt` — initial formatting gate.
- `release-validation-check-fmt-post-candidate.txt` — formatting gate after the
  test-only matrix.
- `release-validation-vet.txt` — root plus 12 standalone modules.
- `release-validation-go-fix-diff.txt` — root `go fix -diff`, empty.
- `release-validation-check-fix.txt` — full recipe, expected exit 1 at dirty
  `git diff`.
- `release-validation-check-fix-gofix.diff` — 0-byte `/tmp/go-fix.diff` from
  that recipe.
- `release-validation-lint.txt` — pinned golangci-lint 2.14.0, 0 issues.
- `release-validation-lint-post-candidate.txt` — same pinned lint after the
  test-only matrix.
- `release-validation-vulncheck.txt` — pinned govulncheck full recipe.
- `release-validation-govulncheck-version.txt` — v1.3.0 identity.
- `release-validation-test-race-root.txt` — root race suite, 36-package scope.
- `release-validation-test-race-pkg-anduin.txt`,
  `...-pkg-inertia.txt`, `...-pkg-kiks.txt` — affected standalone race runs.
- `release-validation-standalone-modules.txt` — published module resolution,
  no replacements, verify/tidy for the three affected modules.
- `release-validation-standalone-go-fix-diff.txt` — empty standalone fix
  diffs.
- `release-validation-test-upgrade-e2e.txt` — historical single-path beta.4 ->
  beta.5 upgrade E2E (pre-change log).
- `release-validation-test-upgrade-e2e-candidate.txt` — new three-case matrix:
  beta.4 -> beta.5, beta.5 -> rc.1, beta.7 -> rc.1, all PASS.
- `release-validation-upgrade-candidate.diff` — test-only change to
  `layout/upgrade/upgrade_e2e_test.go`.
- `release-validation-upgrade-candidate-compile.txt` — compile/skip check,
  focused pre-release policy tests, full upgrade package tests.
- `release-validation-upgrade-candidate-static.txt` — gofmt, `go fix -diff` and
  `go vet` for the changed package.
- `release-validation-upgrade-candidate-verification.txt` — whole-root
  `go fix -diff`, `just vet`, and `go test ./layout/...`.
- `release-validation-check-release-notes.txt` — notes validator output.
- `release-validation-root-suite-scope.txt` — `test` vs `test-race` package set
  proof.
- `release-validation-final-state.txt` — pre-test final status/diff, unchanged
  from the original baseline, `git diff --check` exit 0.
- `release-validation-final-state-post-candidate-test.txt` — current final
  status with the test-only matrix: 20 tracked files, `git diff --check` exit
  0.

Historical evidence not rerun here: `evidence/pin-integration-test-golden-full-release.txt`
(release-shaped golden PASS), `evidence/pin-integration-module-download.json`
(published anduin/inertia sums) and the package publication records.

# rc.1 post-merge release validation (Narsilc v0.6.4 combined tree)

Status: **POST-MERGE LOCAL GATES PASS ON THE COMBINED TREE. READY FOR
INDEPENDENT REVIEW. NOT YET RC.1-READY / TAGGABLE: THE FINAL MASTER PUSH AND
CI ARTIFACT PREFLIGHT REMAIN PENDING.** No commit, tag, push, publication or
deployment was performed by this validation.

Prepared: 2026-11-02. This is a new post-merge record. It does not overwrite or
replace the historical pre-merge records in `release-validation.md` and
`pin-integration.md`. The other pin worker owns golden regeneration, the full
release golden and pin-focused proxy tests; those were not duplicated here.

## Exact source identity

```text
Branch:       fix/rc1-final-readiness
HEAD:         41657d5dee3f043916c2cb12a3bc4a28ee19082b
HEAD subject: Merge remote-tracking branch 'origin/master' into fix/rc1-final-readiness
HEAD parents: edc738c01b46ab98debe08549b20dce6baeab6d2
              ee84032c62987cdcfca3d93a183709808681ef23
RC1 source:   edc738c01b46ab98debe08549b20dce6baeab6d2
              fix(rc1): close beta7 tooling defects and align runtime pins
Kiks test:    d6f062f99ecfaf67e2a6052da8c1c1cc801fbb31
              test(kiks): verify published router error persistence
origin/master ee84032c62987cdcfca3d93a183709808681ef23
              chore(layout): scaffold narsilc v0.6.4 (#857)
origin/fix/rc1-final-readiness
              c6c724fa8beb0b563fd4966d3d7748af70ad6f46 (merge commit is local)
Go:           go1.27.3 linux/amd64
GOWORK:       off for every just recipe and standalone module run
Proxy/checks: https://proxy.golang.org and sum.golang.org for race, upgrade E2E
              and standalone module runs
Lint:         golangci-lint 2.14.0 (pinned)
Vuln:         govulncheck v1.3.0 from /tmp/opencode/rc1-tools
Race:         CGO_ENABLED=1, CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc'
```

The tracked worktree is clean at this commit. The only untracked paths are
`docs/release-notes/v2.0.0-rc.1.md` and `plans/rc1-final-readiness/` evidence.
`git diff --exit-code -- .` exits 0, so untracked plans/draft notes do not
affect `just check-fix`. Evidence:
`evidence/postmerge-release-validation-baseline.txt`;
`evidence/postmerge-release-validation-final-state.txt`.

## Combined framework pins and published identities

The merged tree combines the released rc.1 runtime pins with the new Narsilc
tool/runtime pin:

| Pin | Current value |
| --- | --- |
| `versions.Narsilc` | `v0.6.4` |
| `versions.NarsilcModule` | `v0.6.4` |
| `versions.Anduin` | `v0.8.0` |
| `versions.Inertia` | `v0.14.1` |
| `versions.Kiks` | `v0.6.1` |

The narsilc lock goldens now carry the v0.6.4 tool hashes (for example
`fe6c31f3...` for linux/amd64). The published runtime tags are intact:

| Tag | Commit |
| --- | --- |
| `pkg/anduin/v0.8.0` | `718076edfafe0735032bf6a2d457b5c09bbd8251` |
| `pkg/inertia/v0.14.1` | `c6c724fa8beb0b563fd4966d3d7748af70ad6f46` |

The public proxy/checksum re-verification and pin-focused tests remain owned
by the other pin worker; they were not rerun here. This record confirms the
exact source pins and published tags above, not a new proxy audit.

## Gate results

| Gate | Command | Exit | Result | Evidence |
| --- | --- | --- | --- | --- |
| Clean go-fix gate | `just check-fix` | 0 | **Pass.** Tracked tree clean; `go fix -diff ./...` empty (`/tmp/go-fix.diff` 0 bytes); final `git diff --exit-code -- .` clean. Untracked plans/draft notes do not count. | `evidence/postmerge-release-validation-check-fix.txt` |
| Root race | `CGO_ENABLED=1 CC='.../zig cc' ... just test-race` | 0 | **Pass.** 36 packages selected (all root except `golden`); 29 `ok` plus no-test packages. Key times: cli 41.181s, layers 201.228s, layout 17.396s, layout/upgrade 2.804s. | `evidence/postmerge-release-validation-test-race-root.txt` |
| Upgrade E2E | `GOPROXY=... GOSUMDB=... just test-upgrade-e2e` | 0 | **Pass.** All three cases: beta.4 -> beta.5 regression, beta.5 -> rc.1, beta.7 -> rc.1; 19.665s. Candidate metadata/pins/no-replace assertions pass. | `evidence/postmerge-release-validation-test-upgrade-e2e.txt` |
| Pinned lint | `TMPDIR=/tmp/opencode just lint` | 0 | **Pass.** golangci-lint 2.14.0, `0 issues`. | `evidence/postmerge-release-validation-lint.txt` |
| Pinned vulnerability scan | `PATH=/tmp/opencode/rc1-tools:$PATH just vulncheck` | 0 | **Pass.** govulncheck 1.3.0; root plus all 12 `pkg/*` modules; no called vulnerabilities. | `evidence/postmerge-release-validation-vulncheck.txt` |
| Vet | `just vet` | 0 | **Pass.** Root plus all 12 standalone modules. | `evidence/postmerge-release-validation-vet.txt` |
| Standalone race: anduin | `GOWORK=off ... go test -mod=readonly -race -count=1 ./...` | 0 | **Pass.** anduin 1.391s, anduintest 1.036s. | `evidence/postmerge-release-validation-test-race-pkg-anduin.txt` |
| Standalone race: inertia | same, in `pkg/inertia` | 0 | **Pass.** 1.595s. | `evidence/postmerge-release-validation-test-race-pkg-inertia.txt` |
| Standalone race: kiks | same, in `pkg/kiks` | 0 | **Pass.** 3.161s. | `evidence/postmerge-release-validation-test-race-pkg-kiks.txt` |
| Standalone metadata | `go list -m all`, `go mod verify` | 0 | **Pass.** No `replace` entries; anduin v0.8.0 resolved for inertia/kiks; `all modules verified`. | `evidence/postmerge-release-validation-standalone-modules.txt` |
| Formatting (extra) | `just check-fmt` | 0 | **Pass.** Empty output. Extra confirmation; not a pin-worker duplication. | `evidence/postmerge-release-validation-check-fmt.txt` |

`just test` was not run separately. `test-race` selects exactly the same root
package set as `just test` (all root packages except `golden`), so the root
race run covers the full root test set with the stronger check. The full
release golden and pin-focused proxy tests were deliberately not duplicated;
they remain with the other pin worker.

## Root race detail

The race run used Zig 0.14.1 as the C compiler with `CGO_ENABLED=1`, and the
justfile exported `GOWORK=off`. Public proxy/checksums were explicit. All root
packages except the heavyweight `golden` package passed. The largest observed
package was `cli/internal/layers` at 201.228s; `cli` was 41.181s and `layout`
was 17.396s. No failure, panic or data-race report appeared.

## Upgrade E2E detail

Command environment: `GOPROXY=https://proxy.golang.org`,
`GOSUMDB=sum.golang.org`, empty private-module overrides, `GOWORK=off` (from
the justfile), disposable `TMPDIR=/tmp/opencode`.

The current test matrix in `layout/upgrade/upgrade_e2e_test.go` ran:

```text
--- PASS: TestUpgradeToHEAD (19.66s)
    --- PASS: TestUpgradeToHEAD/v2.0.0-beta.4-to-v2.0.0-beta.5 (6.66s)
    --- PASS: TestUpgradeToHEAD/v2.0.0-beta.5-to-v2.0.0-rc.1 (6.07s)
    --- PASS: TestUpgradeToHEAD/v2.0.0-beta.7-to-v2.0.0-rc.1 (6.93s)
PASS
ok  github.com/mbvlabs/andurel/v2/layout/upgrade  19.665s
EXIT: 0
```

The beta.5/beta.7 candidate paths are scaffolded with the published CLIs and
upgraded by this checkout's upgrader at target `v2.0.0-rc.1`; no published rc.1
CLI is implied. Candidate assertions after tidy require the rc.1 manifest
version, `anduin v0.8.0`, `inertia v0.14.1` and no `replace` directives. They
all pass, and build/vet complete in the disposable app.

With the combined Narsilc v0.6.4 pin, every case now also reports the expected
managed tool metadata update:

```text
Updating managed tool metadata...
  Updated:
    narsilc: v0.6.4
```

No Docker, database or production service was used; the test owns only its
temporary scaffold. The actual CLI `--json` command-surface behavior is covered
by the accepted CLI tests; it was not duplicated in this E2E test.

## Vulnerability detail

The pinned scanner (not the ambient newer install) completed root and every
standalone package. It reported `No vulnerabilities found` for each module and
`Your code is affected by 0 vulnerabilities` where symbol analysis applied.
As before, some required module versions have uncalled advisories; this is not
a claim that every transitive advisory is absent.

## Scope not performed here

- No golden regeneration, no full `test-golden-full-release`, and no
  pin-focused proxy test run; those are owned by the other pin worker. The
  historical pre-merge full-golden PASS was Narsilc v0.6.3 scope and is not
  evidence for this v0.6.4 combined tree.
- No site T09/T07, deployment, image, database or production action.
- No code, test, pin, lock or golden edit by this validation. The tracked tree
  is the owner/parent commit `41657d5d`; this task only added report/evidence.
- No commit, tag, push, PR merge or publication.

## Pending final CI gate

The final master gate remains pending because the merge commit is still on
`fix/rc1-final-readiness`, not pushed to `master`. A successful push-triggered
`Release Readiness` run for the exact final master SHA, including its
`Release artifact preflight` job, is still required. Local runs above are not a
substitute and no CI success is claimed.

After the owner pushes/merges the PR, verify the exact master SHA with the
workflow record (same check documented in `publication-handoff.md` and the
pre-merge report). The overall rc.1 verdict remains **not ready** until that CI
gate, the pin worker's refreshed golden/pin evidence, site T09/T07 as
applicable, final release notes and the owner go/no-go all complete.

## Evidence index

All new post-merge evidence is under
`plans/rc1-final-readiness/evidence/`:

- `postmerge-release-validation-baseline.txt` — HEAD/parents/tags, pins, Go and
  tool identity, initial clean tracked status.
- `postmerge-release-validation-check-fix.txt` — `just check-fix` exit 0 and
  0-byte go-fix diff.
- `postmerge-release-validation-check-fmt.txt` — extra formatting pass.
- `postmerge-release-validation-vet.txt` — root plus 12 modules.
- `postmerge-release-validation-lint.txt` — pinned 2.14.0, 0 issues.
- `postmerge-release-validation-vulncheck.txt` — pinned 1.3.0 scan.
- `postmerge-release-validation-test-race-root.txt` — Zig/CGO root race.
- `postmerge-release-validation-test-race-pkg-anduin.txt`,
  `...-pkg-inertia.txt`, `...-pkg-kiks.txt` — affected standalone races.
- `postmerge-release-validation-standalone-modules.txt` — module versions,
  no replacements, verify.
- `postmerge-release-validation-test-upgrade-e2e.txt` — three-case upgrade
  matrix, including narsilc v0.6.4 tool metadata update.
- `postmerge-release-validation-final-state.txt` — final clean tracked state.

Historical pre-merge records and evidence are preserved unchanged.

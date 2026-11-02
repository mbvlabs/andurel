# rc.1 execution ledger

Approved spec: `/home/mbv/work/andurel/specs/rc1-final-readiness.md`.

## Latest checkpoint (supersedes historical status prose below)

- Published and verified: anduin v0.8.0 (`718076ed`), inertia v0.14.1 (`c6c724fa`); kiks stays v0.6.1.
- Reviewed source is committed and master PR #857 merged normally at `41657d5d` (tree `f2f56478`): narsilc generator/runtime v0.6.4 plus released runtime pins.
- All framework local/post-merge release gates independently accepted: full release goldens, zero drift, public pins, module checks, check-fix, vet, lint, vulnerability scans, root/standalone races and historical/candidate upgrade E2Es.
- Published standalone runtime integration accepted; updated site pins and authenticated-error regression/full validation accepted.
- Still pending: final updated-runtime site live/image/docs parity evidence/acceptance, exact final-master CI/readiness/artifact preflight, owner rc.1 go/no-go. **Not ready to tag rc.1.**
- No history rewriting or published-tag moves; owner performed package pushes. New local branch commits/evidence require the owner's next branch push/PR.

The remaining sections preserve the execution chronology, including earlier blocked/superseded checkpoints; consult this latest checkpoint and the post-merge reports for current status.

Evidence archival note: files under `evidence/` are preserved command output,
including literal unified diffs and intentionally failed historical checks.
Those diffs contain context-prefix spaces followed by Go tabs, and some raw
output has trailing spaces. `pin-integration.md` and
`kiks-published-validation.md` also quote those literal diffs. Do not rewrite
the snapshots to satisfy prose whitespace checks; validate source/current
documentation separately. No repository whitespace policy has been changed.

## T00 baseline

- Framework: `/home/mbv/work/andurel-rc1-work`, branch `fix/rc1-final-readiness`, baseline `289b1778de20f86210dbec749a1c622ff6018d65`.
- Site: `/home/mbv/work/andurel-site-rc1-work`, branch `fix/rc1-acceptance`, baseline `7431d444b04e436912ee95d6ec531184fdfc871f`. This is newer than planning's site snapshot; inspect current evidence rather than assuming old pending details unchanged.
- Baseline scout confirms the newer site's `plans/beta7-port/review.md` records historical independent T07 acceptance. Preserve that accepted historical result; candidate safety changes still need scoped independent review/reruns. Do not describe historical T07 acceptance as absent on this newer tree.
- Both worktrees started clean. Original repositories and their existing untracked work are preserved.
- Go: `go1.27.3 linux/amd64`; x/tools pin `v0.52.0`; development workspace `/home/mbv/work/andurel-rc1-work/go.work`.
- Proxy: `https://proxy.golang.org,direct`; checksum DB: `sum.golang.org`.
- Installed CLI: `/home/mbv/.local/share/mise/installs/go/1.27.1/bin/andurel`, reports `v2.0.0-beta.7 (built: 2026-11-02)`. This is not the candidate binary.
- Framework default pins: anduin `v0.7.1`, narsilc tool/runtime `v0.6.3`.

## Ownership and sequence

| Task | Status | Owner / evidence |
| --- | --- | --- |
| T00 baseline/reproductions | Complete | Baseline plus red/green evidence in `t01.md`–`t04.md` and coordinated T02 evidence |
| T01 narsilc diagnostics | Independently reviewed; no actionable findings | `t01.md`; reviewer independently passed full CLI tests/vet, focused matrix, fix/format checks and command metadata check. Non-mismatch JSON states use report/inspector matrix coverage rather than separate end-to-end runs. |
| T02 middleware context + B7-007 | Independently accepted locally; all findings closed | Final cancellation publication barrier reviewed with full tests/vet and five race repetitions. Frozen timeout classification, child propagation, nil masking/cause, error identity, nested scopes, buffers/headers/flush verified. Published proof pending. |
| T02 inertia capture | Independently accepted; all findings closed | 405/security/CORS/retry/auth metadata preserved; rendered reflash-failure delivery fixed; actual RateLimit/401 regressions and reviewer module tests/vet pass. Published anduin requirement update remains gated. |
| T02 kiks persistence | Independently accepted including both P3 improvements | Reviewer confirms global409 cookie roundtrip and direct pre-consumer timing/status200 controls. Worker full stable-anduin tests/race/vet pass. No kiks runtime/pin changes. |
| T02 combined development integration | Independently accepted, development scope only | Reviewer freshly passes standalone real-module tests10x/readonly/race/vet/fix/format; strict authenticated provider, resource/cancel, cookies/flash, capture, JSON consumers and nested retries substantive. NOT published proof. |
| T09 standalone published-runtime integration | Independently accepted; no findings | Reviewer confirms all five public origins/checksums, byte-identical11 tests, no replacement/workspace leakage; readonly10x/race/vet/tidy/moduleverify/fix/format pass. See `t09-published-integration.md`. Not full framework/site CI. |
| T03 upgrade JSON + B7-003 | Independently reviewed; accepted, no remaining findings | Execution-local recorder and failed-error metadata validated; reviewer full CLI/domain tests/vet/format/diff pass. Worker Zig race passes recorded in `t03.md`; reviewer did not repeat with Zig. |
| T04 synthetic test main | Independently reviewed; accepted, no remaining findings | Exact cache-artifact provenance preserves root/ancestor-cache source and symlink controls. Reviewer independently passed full layers tests/vet/format/diff; worker final-source race passes recorded in `t04.md`. |
| T05/T06 site harness | Independently reviewed; accepted, no blocking findings | Reviewer reran args 26/26, redaction 44/44, assets 16/16, additional escaped-value controls, syntax/shellcheck/diff. Safe for isolated candidate live/image testing. |
| Independent code review | Tooling/harness accepted; coordinated anduin review and inertia/kiks work in progress | Reviewer, not fixing workers |
| T07 live/image acceptance | Independently accepted: baseline published-runtime harness evidence | Reviewer verified live/image identities, all 342 manifest entries, no mandatory skips, safe cleanup and unchanged historical evidence. Site `t07.md`. Not expanded-runtime T09 or release approval; queue is startup/shutdown, not SMTP/delivery qualification. |
| T09 site published-runtime pins/regression | Independently accepted | Only anduin/inertia pins+sums and real demotedadmin authenticated403 test; 71file manifest verified. Full verify9stages375tests516subtests0skips, race/editor/static/parity/idempotence pass. Post-pin live/image/docs-server parity rerun delegated. |
| T08 dependency publication/pins | Post-master-merge pins/goldens independently accepted | Reviewer accepts41657d5d/treef2f56478, masterNarsilc0.6.4 exact4hashes plus Anduin0.8.0/Inertia0.14.1/Kiks0.6.1, all439golden hashes stable regeneration. Tags immutable, focusedCLI/layout/vet/fix/format pass; fullreleasegolden/publicpins evidence sound. |
| Release handoff / notes | Draft reviewed; kiks timing correction applied | `publication-handoff.md`, `docs/release-notes/v2.0.0-rc.1.md`; release-notes validator passes. Suggested versions only, not authorization or final release notes. |
| T09 local framework release gates | Independently accepted at41657d5d | Fullreleasegolden, public12pins, cleancheckfix, root+standalonerace/vet/lint/vuln and threeactualupgradecases accepted with exactsource evidence. FinalmasterCI/artifactpreflight + finalsite liveimage acceptance + owner go/no-go still pending. No release-ready claim. |

Workers must record red/green results before declaring fixes. Local workspace proof is not published-pin release proof. No global gate is passed merely because work was launched. Preserve historical beta7 T08 evidence; new changes require fresh validation.

### Execution clarification from T03 review

The approved spec's dry-run “no execution” wording cannot literally exclude existing read-only `git status --porcelain` safety checks. Preserve those checks and report actual command execution truthfully; dry runs must not perform mutation/install/tidy subprocesses. An empty command list is valid only when no subprocess actually ran. This narrows ambiguous wording without expanding mutation permissions.

### Race toolchain

Default `CGO_ENABLED=0` and no gcc do not prevent race validation: `/home/mbv/.local/share/mise/installs/zig/0.14.1/zig` is installed (version confirmed). Use `CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc'`; the unconfigured `zig` shim alone does not work. Do not change global tool settings.

Executed `CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' go test -race ./cli/internal/layers -count=1`: **PASS**, 164.670s. This run predates completion of the T04 review corrections; rerun on the final accepted source before treating it as final-tree evidence.

Executed with the same Zig/CGO environment: `go test -race ./cli -run 'NarsilcRuntime|SetVersionNarsilc|Doctor' -count=1`: **PASS**, 1.799s. This supplements the independent T01 review; full integrated final-tree gates remain pending.

### Development binary

`go build -o /tmp/opencode/andurel-rc1-candidate .`: **PASS**. Binary reports `v2.0.0-beta.7+dirty (built: 2026-11-02)`; it is a development snapshot, not a released rc.1 binary. T02 remains unaccepted and no release identity is implied.

### Published baseline pins

`just check-published-pins`: **PASS**, all 12 existing package pins verified, including anduin v0.7.1. This establishes current published baseline availability only: it does not verify the unaccepted local anduin changes, publish them, or pass the post-fix T08/T09 gates.

### Integrated validation in progress

`just check-fmt`: **PASS**. `just check-fix` stops with exit 1 at its final `git diff --exit-code -- .` because intentional implementation changes are uncommitted; its preceding `go fix -diff` emitted no suggested fixes. This recipe requires a clean committed tree, so do not label the entire recipe passed. Downstream chained module/vet/race commands did not run in that attempt.

Separate retry completed with exit 0: `go fix -diff ./...` emitted no changes, `just check-mod` verified all modules with no tidy drift, `go vet ./...` passed, and root `go test -race ./... -count=1` passed using Zig/CGO. Includes CLI (36.118s), layers (156.483s), golden package (32.272s) and upgrade domain (3.553s). These root-module results do not include standalone `pkg/anduin` or replace the explicitly release-shaped golden E2E/published-runtime proof. The historical T02 design-hold run predates the coordinated replacement; see `t02-coordinated.md` for current module evidence.

### Late external metadata research

Go/packages documents IDs as opaque and `ForTest` is not a reliable synthetic-main discriminator. Go's generated-test source/header/layout are implementation details, not a stable public contract: <https://pkg.go.dev/golang.org/x/tools/go/packages>, <https://github.com/golang/go/blob/master/src/cmd/go/internal/load/test.go>. Research based on upstream master suggested `_testmain.go`/outside-root paths but did not execute the pinned toolchain; those suggestions do not apply to this reproduced in-root cache case. Prefer T04's executed Go 1.27.3/x-tools v0.52.0 characterization and conservative artifact controls. Revalidate when supported Go versions change; never restore root-containment filtering from unexecuted upstream assumptions.

### Pinned lint and vulnerability gates

- `TMPDIR=/tmp/opencode just lint`: **PASS**, pinned golangci-lint 2.14.0 reports 0 issues. Recipe intentionally uses `GOWORK=off`; this is root-module scope, not all standalone runtime modules.
- Installed govulncheck **v1.3.0** into isolated `/tmp/opencode/rc1-tools` (existing PATH scanner is v1.8.0, not workflow pin). Pinned `just vulncheck` root and anduin/auth/cache/email/env scans find no called vulnerabilities. Recipe then **BLOCKED** at inertia: its published anduin requirement lacks new `ResponseBuffer`/`RenderedError` APIs. This is the intentional publication/pin gate, not a vulnerability finding or passing full scan. Remaining recipe modules did not execute. Some scanned required modules contain uncalled vulnerabilities; scanner reports 0 affected code symbols, not blanket absence of all module advisories.
- Separate explicit-workspace scans of changed anduin/inertia/kiks modules: **PASS**, pinned govulncheck reports no vulnerabilities for each. Development diagnosis only; cannot replace the post-publication recipe.

**Post-publication full recipe:** pinned govulncheck v1.3.0 `just vulncheck` now **PASS**, exit 0 across root and every standalone package with `GOWORK=off` and published dependencies, including inertia/kiks. No called/imported vulnerable symbols found. Some required module versions have uncalled advisories; do not describe this as absence of every transitive module advisory. This supersedes the earlier missing-API scan blocker.

### Combined orchestrator rerun

After coordinated module corrections, orchestrator sequentially ran vet and Zig/CGO race suites for anduin (including anduintest), inertia, kiks, standalone integration (`GOWORK=off`, `-mod=readonly`) and the complete root module: **all PASS**, exit 0. Root CLI 34.078s, layers 153.291s, golden package 27.774s. This run began before the final timeout-`Err` refinement was delegated; affected runtime modules/integration must be rerun after that last change. Root tooling results remain useful; release-shaped published-runtime proof is still unavailable until authorized module releases/pin updates.

Final post-frozen-cancellation rerun: orchestrator vet and Zig/CGO race suites **all PASS** for anduin/anduintest (1.394s/1.036s), inertia (1.630s), kiks (3.155s), and standalone integration repeated 10 times with `GOWORK=off -mod=readonly` (1.383s); `git diff --check` passes. This supersedes earlier runtime reruns for executed local-source validation. Independent final cancellation review remains pending; published dependency gates unchanged.

Final post-publication-barrier rerun (supersedes runtime timings above): **all PASS**, anduin/anduintest 1.408s/1.035s, inertia 1.641s, kiks 3.168s, standalone integration 10x race/readonly 1.294s; each module/integration vet and final diff check pass. Separate reviewer closes the last P2 and accepts anduin local implementation. No remaining known local review blocker.

## Current verdict and stop point

**Local implementation and independent review complete; NOT rc.1-ready or eligible to tag.** All four framework defects have local fixes and regression evidence. Coordinated runtime changes are development-tested only until the separate modules are published and consumed through public pins. Harness safety and baseline-runtime live/image acceptance pass independently; that does not replace new-runtime site acceptance.

Next owner-controlled gate: authorize/perform package publication in dependency order described in `publication-handoff.md` (anduin first, then inertia; kiks release optional for test/requirement refresh). Suggested versions are recommendations only. No commits, pushes, tags, publication or deployments were performed in this session. After authorized package releases become visible, update pins/goldens through supported tooling, rerun public-proxy/checksum release-shaped verification/artifact preflight and site T09, then reconsider the rc.1 verdict. Draft release notes must not be finalized before those gates.

**Subsequent owner instruction:** package release and version-pin work is now authorized. Proceed with anduin v0.8.0 and inertia v0.14.1; retain kiks framework pin v0.6.1. Anduin standalone/public-dependency release preflight passes including tests/race/vet/build/tidy; package-cut reviewer pending. Prepare local commits/tags only: owner performs pushes under standing git instructions. Dependent pin changes wait for actual public proxy/checksum visibility, not local tag existence.

**Anduin checkpoint:** release reviewer accepted all ten source blobs and independently repeated standalone public-dependency checks. Created package-only Conventional Commit `718076edfafe0735032bf6a2d457b5c09bbd8251` (`feat(anduin): release v0.8.0`) and local tag `pkg/anduin/v0.8.0`. All other work remains separate/uncommitted. No push/publication performed. Owner push/public visibility is now the blocking prerequisite for inertia release and dependent version pins.

## Master integration update

Owner merged PR #857; fetched `origin/master` at `ee84032c` (from baseline `289b1778`). Master now sets narsilc generator/runtime v0.6.4 with authoritative lock digests and scaffold goldens. Earlier “keep framework narsilc v0.6.3” instructions describe the prior baseline only and are superseded by this accepted master change. Preserve site v0.6.4 alignment.

After active source workers finish and intended local changes are committed, integrate via normal `git merge origin/master` (no stash/rebase/amend/tag moves), combine narsilc0.6.4 with anduin0.8.0/inertia0.14.1 and existing runtime guidance, regenerate all affected golden expectations with supported tooling and rerun affected full checks. Published anduin/inertia tag commits remain immutable. Until integration/reruns complete, earlier framework golden/e2e results refer to the pre-merge source, not the final combined candidate.

**Integration executed:** committed independently reviewed kiks tests/metadata as `d6f062f9` and framework tooling/pins/upgrade matrix as `edc738c0`; normal merge succeeded automatically at `41657d5dee3f043916c2cb12a3bc4a28ee19082b`. Combined constants are narsilc tool/runtime0.6.4, anduin0.8.0, inertia0.14.1, kiks0.6.1. Published tags remain at their original commits. Tracked framework source is clean; plans/draft notes remain untracked pending reconciliation. Fresh post-merge full goldens/pins and release validation are running; whole-tree `check-fix` can now run without the earlier intentional-dirty-source failure. No remote push of these new commits was performed here.

**Post-merge executed gates:** `post-merge-pins.md` records all439 golden hashes stable through regeneration, full release-shaped golden PASS133.537s, public pins12/12, mod/fmt/checkfix/vet, full CLI/layout/focused skew tests PASS; master4narsilc digests verified against actual release artifacts. `postmerge-release-validation.md` records wholecheckfixPASS, pinnedlint0issues, pinnedvulnroot+12 no called findings, vetroot+12, rootrace36pkgsPASS and affectedstandaloneraces publicreadonlyPASS, actual3upgradeE2E casesPASS19.665s (including beta5/beta7->rc1). No tracked source changes. Separate merged-pin and gate-evidence reviewers are now checking these results. CI exact finalmaster/artifactpreflight and final site live/image acceptance remain pending.

# Post-merge rc.1 pin and golden revalidation

Status: **REVIEW-READY FOR THE MERGED PIN/GOLDEN SOURCE. NO SOURCE OR GOLDEN
CHANGE WAS NEEDED. RELEASE-SHAPED FRESH-SCAFFOLD GOLDENS PASS ON THE MERGED
TREE.** This is pin/golden revalidation only, not a release verdict. No commit,
rebase, amend, tag, push, publication or deployment was performed.

Prepared: 2026-11-02. This record covers the normal merge of accepted rc.1 work
with `origin/master` #857. It extends the pre-merge `pin-integration.md` and
`release-validation.md`; those files are historical and were not overwritten.

## Merged identity

```text
Worktree:       /home/mbv/work/andurel-rc1-work
Branch:         fix/rc1-final-readiness
HEAD:           41657d5dee3f043916c2cb12a3bc4a28ee19082b
HEAD tree:      f2f56478c84bb8fe53d2530e16c186a13576ea43
HEAD subject:   Merge remote-tracking branch 'origin/master' into fix/rc1-final-readiness
Merge parent 1: edc738c01b46ab98debe08549b20dce6baeab6d2 (accepted rc.1 work)
Merge parent 2: ee84032c62987cdcfca3d93a183709808681ef23 (origin/master, narsilc #857)
Tracked diff:   empty
git status:     only untracked docs/release-notes/v2.0.0-rc.1.md and plans/rc1-final-readiness/
git diff --check: exit 0
```

Exact merged source hashes:

| File | SHA-256 | Git blob |
| --- | --- | --- |
| `layout/versions/versions.go` | `2547019078499ff87971c5d6f60e60f6c855d88fefc41820910f563a8f4fd5a0` | `d66982e439adcd16dc2d9cafeb0e90e425a68f3f` |
| `layout/lock.go` | `c769514c588ece54cefa149d7201c12ef719698c60ba6dab5d47f8de6ea420ad` | `38f3d4010e325ca15592b0f1d25b61eabacbb2b1` |
| `cli/lock.go` | `caa06df2b47e3c21edd1bf8b2e3e3c29318841db435227a0de87ee2ef4112ddf` | `30a4b60b900b952d68372c2937c76246d82aa5de` |

`layout/lock.go` in the merged tree is byte-identical to
`git show ee84032c:layout/lock.go` (`c769514c...`), proving the four narsilc
platform digests came from accepted master unchanged. Full identity output:
`evidence/postmerge-source-identity.txt`.

## Merged pins

`layout/versions/versions.go`:

| Constant | Merged value |
| --- | --- |
| `Narsilc` | `v0.6.4` |
| `NarsilcModule` | `v0.6.4` |
| `Anduin` | `v0.8.0` |
| `Inertia` | `v0.14.1` |
| `Kiks` | `v0.6.1` |
| `Routing` | `v0.9.1` |
| `Telemetry` | `v0.2.3` |

`layout/lock.go` narsilc tool release digests:

| Platform | SHA-256 |
| --- | --- |
| `linux/amd64` | `fe6c31f3d70412590e55e829f5b2c877abc20b515619889cbaa7d3c466a4aeaa` |
| `linux/arm64` | `acd110d0fb533bff768218880a2f21ca912a5df9c9baf27a7832122952ca52a4` |
| `darwin/amd64` | `2ec77193c74a99d8b3c0027ef63a64ea36396280ecc372c9d89bf132d9d32b02` |
| `darwin/arm64` | `fbe47d6ed74c8dab4c16c511f7a5b8e491afac673769207c9a8460d05aaf0011` |

All four v0.6.4 release tarballs were downloaded from the narsilc GitHub
release and hashed locally; every digest matched the master values above.
Output: `evidence/postmerge-narsilc-asset-hashes.txt`. The golden suite's
`TestMain` additionally downloads and verifies the linux/amd64 narsilc artifact
through these lock defaults on every run.

The `andurel tool set-version narsilc 0.6.4` guidance and the runtime-mismatch
explanation in `cli/lock.go` are retained. `set-version` still never edits
`go.mod`/`go.sum`; it reports the exact `go get ...@v0.6.4` plus
`go mod tidy` remediation.

Public generator module proof:

```text
github.com/mbvlabs/narsilc v0.6.4
Sum:       h1:j4eBy7SGZSMECFkQjc0megIr7+tfckxYVPdSiA7Znko=
GoModSum:  h1:M+ffO0t2stlnmlvh6WrJr/oFdM36CQjjiVeqATa6jz0=
Origin:    cf06ce13273079b8d95ae1522180e3a2f0c6d7b4 (refs/tags/v0.6.4)
```

Command: `GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org go
mod download -json github.com/mbvlabs/narsilc@v0.6.4`, exit 0, empty stderr.
Full JSON: `evidence/postmerge-narsilc-module-download.json`.

## Golden revalidation

`just update-golden-full` was run after the merge through the supported recipe.
The golden harness forces `GOWORK=off`, published mode uses
`GOPROXY=https://proxy.golang.org,direct` and `GOSUMDB=sum.golang.org`, and no
local-package substitution was used.

```text
$ just update-golden-full
PASS
ok  github.com/mbvlabs/andurel/v2/golden  96.324s
EXIT: 0
```

Log: `evidence/postmerge-update-golden-full.txt`.

Result: **zero golden drift**. All 439 golden file hashes are identical before
and after the update; `evidence/postmerge-golden-drift.diff` is empty and
`git status` still shows no tracked changes. The merged master goldens therefore
already match what this source generates.

Current golden state:

- all three `new/postgresql-inertia-*/andurel.lock.golden` files contain narsilc
  `v0.6.4` with the four platform hashes above and no `v0.6.3` entries;
- all three `new/postgresql-inertia-*/andurel.toml.golden` files pin
  `narsilc = 'v0.6.4'`;
- all three `new/postgresql-inertia-*/go.mod.golden` files contain
  `github.com/mbvlabs/narsilc v0.6.4`, `anduin v0.8.0`, `inertia v0.14.1` and
  `kiks v0.6.1`;
- `andurel.lock.golden` tool digests match `layout/lock.go` exactly.

Release-shaped confirmation with fresh isolated caches, public
proxy/checksums, `GOWORK=off` and no `-update`:

```text
$ just test-golden-full-release
PASS
ok  github.com/mbvlabs/andurel/v2/golden  133.537s
EXIT: 0
```

Log: `evidence/postmerge-test-golden-full-release.txt`.

## Verification matrix

All commands ran on merged HEAD `41657d5d` with a clean tracked tree.

| Command | Result | Evidence |
| --- | --- | --- |
| `just update-golden-full` | PASS, 96.324s, no drift, 439/439 golden hashes unchanged | `evidence/postmerge-update-golden-full.txt` |
| `just test-golden-full-release` | PASS, 133.537s, fresh caches/public proxy/checksums, `GOWORK=off` | `evidence/postmerge-test-golden-full-release.txt` |
| `just check-published-pins` | PASS, 12/12 public pins, including anduin `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1` | `evidence/postmerge-check-published-pins.txt` |
| `just check-mod` | PASS, `all modules verified`, empty tidy diff | `evidence/postmerge-check-mod.txt` |
| `just check-fmt` | PASS, empty output | `evidence/postmerge-check-fmt.txt` |
| `just check-fix` | PASS, whole-tree go fix diff empty and tracked `git diff --exit-code` clean | `evidence/postmerge-check-fix.txt`; `...gofix.diff` |
| `go fix -diff ./...` | PASS, empty diff, empty stderr | `evidence/postmerge-go-fix.diff`; `...stderr.txt` |
| `just vet` | PASS, root plus all 12 standalone `pkg/*` modules | `evidence/postmerge-vet.txt` |
| `go test ./cli -run 'Narsilc\|Doctor' -count=1 -v` | PASS, all focused narsilc/doctor cases and subtests | `evidence/postmerge-cli-narsilc-doctor.txt` |
| `go test ./cli -count=1` | PASS, full CLI package, 28.227s | `evidence/postmerge-test-cli-full.txt` |
| `go test ./layout/... -count=1` | PASS, layout 15.173s, blueprint, cmds, upgrade 1.999s; versions/templates have no tests | `evidence/postmerge-test-layout-all.txt` |
| `git diff --check` | PASS, exit 0 | command output |

## Intentional `v0.6.3` mismatch fixtures

These are deliberate mismatch cases, not stale defaults, and were left intact:

- `cli/doctor_test.go` `TestCheckNarsilcRuntimeStates/mismatched` uses tool
  `v0.6.4` with go.mod `v0.6.3` and asserts the remediation to `v0.6.4`.
- `cli/doctor_test.go` `TestDoctorReportsNarsilcRuntimeMismatchOnBothSurfaces`
  uses the same old-runtime/new-tool mismatch on both human and structured
  surfaces.
- `cli/narsilc_runtime_skew_test.go` uses `v0.6.3` fixtures to prove
  set-version is not silent, does not mutate `go.mod`, and reports exact
  remediation; it also keeps a non-applying `v0.6.3` replacement control.
- `layout/cmds/cmd_test.go` and `layout/upgrade/plan_test.go` use
  `versions.NarsilcModule` dynamically, so they follow the merged `v0.6.4`
  default without literal edits.

No test failed under the merged `v0.6.4` default, so no red/green test edit was
needed and no default reference required correction. No non-test code was
changed, so no owner decision point was reached.

## Historical evidence boundary

The pre-merge `pin-integration.md`, `release-validation.md` and their evidence
correctly describe the `v0.6.3` source at their recorded HEADs. They are
historical and were not overwritten. Their statements that narsilc remains
`v0.6.3` do not describe merged HEAD `41657d5d`; this record is authoritative
for the merged pin/golden tree. Pre-existing `postmerge-release-validation-*`
evidence files in the same directory were not modified by this slice.

## Not run by this pin/golden slice

Remaining full release gates are separate and still required on the exact
candidate: `just lint`, `just vulncheck`, `just test-race`, full
`just test`/`just test-upgrade-e2e`, release-note validation, release-readiness
workflow plus artifact preflight on the pushed master SHA, site T09, published
B7-005 positive proof, and T07 independent live/image acceptance. `just
check-fix` passed here because the tracked tree is clean; it was not skipped for
a dirty-tree exception.

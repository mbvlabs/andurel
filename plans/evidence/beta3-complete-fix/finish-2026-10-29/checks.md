# Resumed beta.3 finishing checks — 2026-10-29

Status: development verification in progress, **not full acceptance**. The
candidate Inertia pin is unpublished. No tags or pushes were performed.

Coordinator checkout: `/home/mbv/work/worktrees/andurel-beta3-fixes/G`, branch
`fix/beta3-complete-integrated`. Started at `0d6cb0ba`; subsequent coordinator
commits are `5e9f8221` and `396baf95`.

## Commands and observed results

Unless noted, Go commands used `TMPDIR=/tmp/opencode`,
`GOTMPDIR=/tmp/opencode`, and `GOWORK=off`.

| Command | Result |
| --- | --- |
| `gofmt -l cli internal layout pkg/inertia scripts/acceptance` | Exit 0; no output. |
| `go fix -diff ./...` | Exit 0; no output, repeated after corrections. |
| `go vet ./...` | Exit 0; repeated after corrections. |
| `go test ./... -count=1 -timeout 20m` | Initial failure: stale composition-pattern readiness assertion. After correction, all root packages pass; repeated after lint fixes. |
| `ANDUREL_GOLDEN_FULL=1 go test ./golden -count=1 -timeout 45m` | Exit 0; 139.247s. Uses the isolated development candidate module proxy, not published-module proof. |
| `go test ./... -race -count=1 -timeout 10m` in `pkg/inertia` | Initial invocation blocked by disabled CGO. Re-run with `CGO_ENABLED=1` and Zig CC: exit 0, 1.604s. Module formatting, fix, and vet checks also clean. |
| `go test ./cli -run 'TestPorts\|TestVerifyPortOwnedByChild\|TestRouteUsage\|TestRecoverInterrupted\|TestBuild' -race -count=1 -timeout 15m` | With CGO/Zig CC: exit 0, 20.621s. |
| `ANDUREL_UPGRADE_E2E=1 ANDUREL_UPGRADE_E2E_LOCAL_PKG=1 go test ./layout/upgrade -run TestUpgradeToHEAD -count=1 -v -timeout 45m` | Exit 0, 47.323s. Alpha.4/alpha.5/beta.1/beta.2 source apps compile/vet. **Existing test targets beta.3, so this does not prove the new beta.4 manual actions**; separate worker addresses this gap. |
| `just lint` | Initially 14 recovered implementation issues; after explicit cleanup, exit 0, `0 issues.` |
| `just check-release-notes v2.0.0-beta.4` | Exit 0; candidate notes file found. |
| `scripts/beta3-fix-acceptance.sh --mode dev --evidence-dir /tmp/opencode/beta3-coordinator/dev-baseline` | Exit 0 but explicitly partial: 16 passes, gates 17/18 blocked. Candidate `5e9f8221`; not full acceptance. |

Race-test compiler: `CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc'`.
Coordinator raw baseline output is retained at
`/tmp/opencode/beta3-coordinator/`; the original durable 19-ID matrix remains in
`../653de5af95f5-dev/matrix.md`. Neither baseline matrix is a final result for
subsequent finishing changes.

## Still outstanding

- Integrate and independently verify the finishing workers' commits.
- Live app/SSR/build, site adoption, hygiene/doctor, actual DB-test evidence.
- Beta.4 manual-action compile coverage and a refreshed 19-ID final matrix.
- Published-module/release-shaped proof after authorized publication. Do not
  fetch the unpublished candidate version merely to demonstrate that it fails.

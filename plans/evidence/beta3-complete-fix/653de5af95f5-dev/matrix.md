# Beta.3 complete-fix evidence matrix (candidate 653de5af)

Acceptance state: **partial**. The dev harness run passed 16 probes, blocked 2,
and failed 0. Blocked gates are not passes. Candidate pin
`pkg/inertia v0.13.0` is unpublished, so no release-shaped proof exists.

Evidence: `probes.md`, `probes.tsv`, `environment.txt`, `candidate-commit.txt`,
`candidate-pins.txt`, `acceptance-state.txt`, and per-probe stdout/stderr/status
in this directory.

| ID | Status | Implementation commit | Development evidence | Release-shaped evidence | Negative/regression probe | Remaining blocker |
| --- | --- | --- | --- | --- | --- | --- |
| F01 | implemented; live blocked | d76640ba, 653de5af | public home template + generated runtime tests; full golden vet | blocked (no published pin) | `TestGeneratedHomePageIsPublic`, `TestAuthOnlyStillProtectsRoutes` | live anonymous `/` gate 17 |
| F04 | implemented | d76640ba, 653de5af | probes 02, 03, 07, 08 | blocked | untouched scaffold setup failure, not drift | none in dev |
| F06 | implemented; live partial | d76640ba, 4865319a | probe 04, `TestCheckRuntimeConfiguration*` | blocked | helper build error fails; missing toolchain skips | live prerelease/network/DB negatives not automated |
| F07 | implemented | e12e018b | probe 01, inspect view tests/goldens | blocked | absent config unresolved, not invented | none |
| F10 | implemented | e12e018b | probe 01 `commands --check` | blocked | contradictory metadata fixture test | none |
| F18 | implemented | 75aa00bb, 994ecb91 | `TestVerifyScope*`, changed tests | blocked | outside-scope violation, invalid base, no-Git | not harness-probed |
| F20 | implemented | 75aa00bb, 994ecb91 | probe 09 | blocked | failing test exits 9; skip retained | real DB uncached tests not run |
| F21 | implemented framework; site blocked | 75aa00bb, 653de5af | probe 08 (missing/duplicate/link) | blocked | each invariant breaks independently with nonzero | site fixtures gate 18 |
| F22 | implemented; release gate blocked | fdfc9166, d76640ba, 653de5af | full scaffold vet with candidate proxy; template assertions | **blocked: v0.13.0 untagged** | `.path()`/deferred allowed; CSR opt-out documented | live production SSR gate 17; publication |
| F25 | implemented static/runtime; live blocked | 454458e3, 589fc43e | probes 11; route-usage alias/IIFE tests | blocked | module-scope/IIFE flagged; `.path()` allowed | two-host live SSR not automated |
| F32 | implemented; live blocked | fdfc9166, d76640ba, 653de5af | readiness template tests; unready schema | blocked | 503 `unready`/`unavailable`, DB reason | sidecar loss/recovery gate 17 |
| F35 | site-only blocked | — | — | — | — | separate site worktree |
| F36 | implemented | e12e018b, 653de5af | probe 01; version consistency tests | blocked | unsupported modes reject as usage errors | beta.4 release notes absent |
| F39 | implemented; live blocked | 2c04fcdc, 653de5af | probes 13-16; isolation/recovery/ownership tests | blocked | interrupted publication repaired; foreign listener rejected | concurrent output builds/live build gate 17 |
| F40 | implemented docs/policy; probe missing | d76640ba | README/AGENTS lockfile + skill policy | blocked | — | hygiene/frozen-install probe not automated |
| N1 | implemented | e12e018b | probe 01 | blocked | unknown inspect nonzero `usage_error`; bare help 0 | none |
| N2 | implemented | e12e018b | probe 06 | blocked | jq `.keys` works | none |
| N3 | implemented | e12e018b | probe 05 | blocked | creation without `.env`/DB | none |
| N4 | implemented; live blocked | 2c04fcdc | compiled generated `cmd/ssr` test | blocked | child failure exits nonzero | occupancy/post-start crash against generated binary gate 17 |

## Next verification commands

```bash
cd /home/mbv/work/worktrees/andurel-beta3-fixes/G
scripts/beta3-fix-acceptance.sh --mode dev
scripts/beta3-fix-acceptance.sh --mode dev --require-all   # must fail while gates 17/18 are blocked
scripts/beta3-fix-acceptance.sh --mode release             # records unpublished pin blocked
go test ./cli ./layout ./layout/upgrade -count=1
go test ./golden -count=1
ANDUREL_GOLDEN_FULL=1 go test ./golden -count=1
CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' CGO_ENABLED=1 \
  go test ./cli -run 'TestPorts|TestVerifyPortOwnedByChild|TestRouteUsage|TestRecoverInterrupted' -race -count=1
```

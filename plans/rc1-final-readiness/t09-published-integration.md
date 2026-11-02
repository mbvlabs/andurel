# T09 published runtime integration

Status: **READY FOR INDEPENDENT REVIEW.** Additional public-module integration
proof for the rc.1 coordinated error lifecycle. No commit, tag, push,
publication, deployment, version bump or application-pin change was performed.

This is standalone published-module proof, **not full framework or site release
CI**. It does not by itself establish the rc.1 release verdict. See
[Limits](#limits-and-non-claims).

Prepared: 2026-11-02.

## Scope and guardrails

- Converted the existing standalone
  `plans/rc1-final-readiness/runtime-integration/` module from a local-replace
  development harness to a public-module proof. The accepted test logic was
  not rewritten.
- Removed every local `replace` with `go mod edit -dropreplace`, pinned the
  published module versions with `go mod edit -require`, and regenerated
  `go.sum` with `go mod tidy` under the public proxy and checksum database.
  No hash was hand-written.
- The previous development report (`t02-integration.md`), its red/green/race
  logs (`evidence/t02-integration-*.txt`) and the pre-conversion module files
  (`evidence/t09-development-*`) are retained unchanged as historical evidence.
- No root, workspace, `shared` or `pkg/*` manifest was modified by this task.
  The pre-existing `pkg/kiks` metadata diff is unrelated accepted work and was
  left untouched.
- No file outside `plans/rc1-final-readiness/runtime-integration/` and the new
  `t09` report/evidence was created or changed.
- No private module override, workspace dependency or local path is active in
  the current proof.

## Identity and environment

Recorded while the shared rc.1 worktree was dirty from other workers:

```text
Framework: /home/mbv/work/andurel-rc1-work
Branch:    fix/rc1-final-readiness
HEAD:      c6c724fa8beb0b563fd4966d3d7748af70ad6f46 (fix(inertia): release v0.14.1)
Anduin:    pkg/anduin/v0.8.0  -> 718076edfafe0735032bf6a2d457b5c09bbd8251
Inertia:   pkg/inertia/v0.14.1 -> c6c724fa8beb0b563fd4966d3d7748af70ad6f46
Go:        go1.27.3 linux/amd64
Module:    andurel.local/rc1/runtime-integration
Location:  plans/rc1-final-readiness/runtime-integration/
GOWORK:    off (explicit)
GOPROXY:   https://proxy.golang.org
GOSUMDB:   sum.golang.org
```

`GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` and `GOFLAGS` were unset. `go env`
reported empty values for all four. Evidence:
`evidence/t09-published-integration-baseline.txt`.

## Published module record

`go mod download -json <module>@<version>` reported the following actual sums;
`go.sum` contains the same values. The origin hashes were independently checked
against local release tags with `git rev-parse <tag>^{commit}`.

| Module | Version | Origin ref | Origin commit | Zip `h1:` | `go.mod` `h1:` |
| --- | --- | --- | --- | --- | --- |
| `github.com/mbvlabs/andurel/pkg/anduin` | `v0.8.0` | `refs/tags/pkg/anduin/v0.8.0` | `718076edfafe0735032bf6a2d457b5c09bbd8251` | `jWpOUeyy2a7DblnBebgg3Hs3S8jC9Af3eaJztzwL8SU=` | `2lRaNyJmrygNQ8lsyHTv4JVTCvnXC7UAWYaqS6mZKMI=` |
| `github.com/mbvlabs/andurel/pkg/inertia` | `v0.14.1` | `refs/tags/pkg/inertia/v0.14.1` | `c6c724fa8beb0b563fd4966d3d7748af70ad6f46` | `TT54ERsQ+IVGEz4/7ZmXVmWcYIM61+YZPjQRdDTRrN8=` | `gWtvWsMb5fM/D0xNZEZ59oLc/o1eViPZw9ZRhUN4RZA=` |
| `github.com/mbvlabs/andurel/pkg/kiks` | `v0.6.1` | `refs/tags/pkg/kiks/v0.6.1` | `774f38bd4309e0d7b4b95039c4125e0d14aeebd4` | `r71gC/jNwbE3IDmJKpQAr7t6uT5E0K8/XaEsHoXuUdI=` | `SOGjsgqIQWTq5O2t/TD1uHhk5Roqd6kn+DIRgJdW+jk=` |
| `github.com/mbvlabs/andurel/pkg/routing` | `v0.9.1` | `refs/tags/pkg/routing/v0.9.1` | `18f329a8ffe1826eddd748e260692d8d20e2dbe6` | `R5wDm/UziR22mfumOWNF7Kekstu20qGaig9aYop5aFQ=` | `tDaz5LVwJXJn733LZhYrsbEWZL7bYNrl4YqQzaBAWpI=` |
| `github.com/mbvlabs/andurel/pkg/telemetry` | `v0.2.3` | `refs/tags/pkg/telemetry/v0.2.3` | `f55963112cb50a84ed9fd7322e94fe0e905d7a33` | `T70mFoL8oKDNByxQt+qbew3GiNVMlqNYBYxknrXWcE0=` | `gcZRxndv7wvMptVsdHCWiRs9jbVWButoNPJsH2axx10=` |

Public proxy checks returned HTTP 200 for `.info`, `.mod` and `.zip` on all
five modules/versions. The authenticated module-cache trees were also compared
with `git archive <origin-hash> pkg/<module>`: all five had no source
differences, and each synthetic module-root `LICENSE` was byte-identical to the
repository root `LICENSE`. Evidence:
`evidence/t09-published-integration-baseline.txt` and
`evidence/t09-published-integration-source.txt`.

## Conversion and module graph

The pre-conversion module and wording were copied to
`evidence/t09-development-go.mod.txt`, `...-go.sum.txt`, `...-README.md` and
`...-doc.go.txt` before editing. Conversion commands, run from the standalone
module with the environment above:

```sh
for module in anduin inertia kiks routing telemetry; do
  go mod edit -dropreplace "github.com/mbvlabs/andurel/pkg/${module}"
done
go mod edit \
  -require=github.com/mbvlabs/andurel/pkg/anduin@v0.8.0 \
  -require=github.com/mbvlabs/andurel/pkg/inertia@v0.14.1 \
  -require=github.com/mbvlabs/andurel/pkg/kiks@v0.6.1 \
  -require=github.com/mbvlabs/andurel/pkg/routing@v0.9.1 \
  -require=github.com/mbvlabs/andurel/pkg/telemetry@v0.2.3
go mod tidy
```

The resulting `go.mod` requires `templ v0.3.1020`, anduin `v0.8.0`, inertia
`v0.14.1`, kiks `v0.6.1` and routing `v0.9.1` directly; telemetry `v0.2.3` is
indirect. MVS resolves routing to `v0.9.1` (ahead of the `v0.9.0` requirement
of anduin/inertia) and anduin to `v0.8.0` (ahead of the `v0.6.2` requirement of
kiks). There is no `replace` directive and no `go.work`/`go.work.sum` in the
module.

```text
$ GOWORK=off go list -m all | grep mbvlabs/andurel
github.com/mbvlabs/andurel/pkg/anduin v0.8.0
github.com/mbvlabs/andurel/pkg/inertia v0.14.1
github.com/mbvlabs/andurel/pkg/kiks v0.6.1
github.com/mbvlabs/andurel/pkg/routing v0.9.1
github.com/mbvlabs/andurel/pkg/telemetry v0.2.3

$ GOWORK=off go mod verify
all modules verified

$ GOWORK=off go mod tidy -diff
(empty, exit 0)
```

`go mod edit -json` prints `"Replace": null`; every listed module resolves
under `/home/mbv/go/pkg/mod/...` rather than the worktree. Evidence:
`evidence/t09-published-integration-module.txt`.

## Test source

The accepted suite is unchanged in behaviour, assertions and bytes. Every test
`.go` file is byte-identical to the pre-conversion snapshot; no test or
assertion was changed. `doc.go` (non-test package documentation) was rewritten
for public mode. No published-module difference required a test change. The
hash comparison is in `evidence/t09-published-integration-changes.txt`; only
`doc.go`, `go.mod` and `go.sum` differ from the hashed pre-conversion snapshot,
while every test source file matches the accepted development harness. The
fixture header's historical "development-only" wording is retained because the
test bytes were not edited; it describes the harness's origin, not current
mode.

The suite has 11 test functions with document / X-Inertia and global / host
variants where relevant. It uses `httptest` only with synthetic hosts,
synthetic cookie/session values and a test renderer root. There is no real
login data, database, network, Docker, production service or application
process.

| Test | Published-module proof |
| --- | --- |
| `TestHostAndGlobalErrorsKeepKiksContext` | Global and host guards, document and X-Inertia. The shared provider is strict: `kiks.Get[*sessionState]` must not return `ErrNoBag`. The ErrorHandler and provider both see the authenticated session, the response is the themed 403 page with the auth prop, and the unreachable success handler cannot leak. |
| `TestEnclosingMiddlewareResourcesAreLiveDuringErrorHandler` | Global and host resource/cancel middleware enclosing the failing guard, document and X-Inertia. The ErrorHandler sees the resource value, `ctx.Err()==nil` and `context.Cause==nil`; cleanup runs exactly once afterwards with the synthetic cancellation cause. |
| `TestDelayedRouteErrorUsesLiveRequestCause` | A route-scope delayed render takes `Err`/`Cause` from the live request even after the request context was canceled with a custom cause before dispatch. |
| `TestErrorRendererBagMutationsPersistWithFinalStatus` | ErrorHandler mutates the real kiks bag and writes 409 or 303, document and X-Inertia. One session save; the dirty session and flash survive a follow-up read. The 403 control keeps the dirty session but emits a cleared flash cookie, and the follow-up sees no flash. |
| `TestErrorRendererFlashStaysOutOfTheResponseBody` | A flash is a cookie concern; it does not appear in the themed body. |
| `TestInertiaCaptureDeliversRenderedErrorExactlyOnce` | Global/host and document/X-Inertia error pages reach the sink with exactly one header write and one body write; the error component appears once and the discarded success component never appears. |
| `TestBufferedSuccessIsReplacedByOuterHostError` | An Inertia success page is buffered, an outer host middleware fails, and the error page replaces it. No stale success body or success-only header is delivered; the base header survives; the final kiks session is saved once with the 403. |
| `TestRouteAPIErrorConsumerReturnsOneJSONResponse` | An `APIDocumentationErrors`-style route consumer returns one JSON 401 or 422 and nil. The router ErrorHandler is never called, no themed page/root is rendered, and delivery is one header write and one body write. |
| `TestNestedCatchErrorsConsumeAndRetry` | A host `CatchErrors` observer defers to an enclosing global consumer; a host `CatchErrors` scope retries once and lets the success page commit. Bag mutations persist on the consumer's final status and the retry commits exactly once. |
| `TestRouteErrorDefersToEnclosingCatchErrors` | A route handler error is carried out of `ServeMux` through request state and consumed by an enclosing global or host `CatchErrors` scope, with one JSON 422, no router render and one persisted session. |
| `TestAuthOnlyStyle303RedirectKeepsKiksCookies` | The AuthOnly-style host middleware destroys the session, stores an encrypted return target and returns a 303 without an error. The destroyed session cookie and return-target cookie persist for document and X-Inertia requests; the follow-up reads the expected admin path. |

## Verification results

All commands used `GOWORK=off`, `GOPROXY=https://proxy.golang.org`,
`GOSUMDB=sum.golang.org` and no private overrides.

| Command | Result |
| --- | --- |
| `gofmt -l .` | empty output, exit 0 |
| `go fix ./...` | exit 0, no source changes |
| `gofmt -l .` after fix | empty output, exit 0 |
| `go vet ./...` | exit 0 |
| `go test -mod=readonly -count=1 -v ./...` | `ok andurel.local/rc1/runtime-integration 0.027s`; all 11 functions and subtests PASS |
| `go test -mod=readonly -count=10 ./...` | `ok andurel.local/rc1/runtime-integration 0.115s` |
| Zig race: `CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' go test -mod=readonly -race -count=1 ./...` | Zig `0.14.1`; `ok andurel.local/rc1/runtime-integration 1.064s` |
| `go mod verify` | `all modules verified` |
| `go mod tidy -diff` | empty output, exit 0 |
| `go mod download` | exit 0 |
| `go list -m all` / `go list -deps -test` | only the five public Andurel versions above; no replacement |
| Manifest hash check for root, workspace and `pkg/*/go.mod`, `pkg/*/go.sum` | all hashes unchanged |

Evidence:

- `evidence/t09-published-integration-baseline.txt` — HEAD, tags, Go, public
  environment, proxy status, origin and sums.
- `evidence/t09-published-integration-module.txt` — `go.mod`, no-replace scan,
  resolved module list, module-cache paths, `go mod verify`, `tidy -diff`.
- `evidence/t09-published-integration-source.txt` — tag archives vs module cache.
- `evidence/t09-published-integration-green.txt` — fmt/fix/vet, readonly
  verbose run and the 10-run repeat.
- `evidence/t09-published-integration-race.txt` — Zig cgo race run.
- `evidence/t09-published-integration-changes.txt` — manifest hashes and the
  source-file hash comparison.
- `evidence/t09-development-*` — preserved pre-conversion module/docs.

## Historical development evidence

The earlier local-replace development proof remains intact and is not presented
as published-dependency or release proof:

- `plans/rc1-final-readiness/t02-integration.md` — unchanged development report.
- `plans/rc1-final-readiness/evidence/t02-integration-baseline.txt`,
  `...-green.txt`, `...-race.txt`, `...-red.txt` — unchanged development logs.
- `plans/rc1-final-readiness/evidence/t09-development-go.mod.txt`,
  `...-go.sum.txt`, `...-README.md`, `...-doc.go.txt` — exact pre-conversion
  snapshots, including the five `replace` directives and DEVELOPMENT-ONLY
  wording.
- `runtime-integration/README.md` — retains a "Historical development mode"
  section pointing at the unchanged development records.

The historical run's local working-tree source is not the current proof.
Current proof is exclusively the public versions and sums recorded above.

## Limits and non-claims

- This is a standalone `httptest`-based integration module. It does not
  validate a real listener, TLS/proxy behaviour, database session stores,
  Docker, deployment or production data.
- It is **not** full framework release CI: no `just` framework gate suite,
  release goldens, scaffold/upgrade E2E, `check-published-pins`,
  `test-golden-full-release` or CI artifact preflight was run here.
- It is **not** site T09 or T07 live/image proof. No application/site was
  upgraded, built, deployed or accepted.
- It proves only that the recorded published module versions interoperate for
  the covered rc.1 error-lifecycle behaviours. It says nothing about later
  source changes, other module versions or the final rc.1 go/no-go.
- No commit, tag, push, publication or deployment was performed.

## Files

Changed/added by this task:

- `plans/rc1-final-readiness/runtime-integration/go.mod` — public requirements,
  no replacements.
- `plans/rc1-final-readiness/runtime-integration/go.sum` — regenerated public
  checksums.
- `plans/rc1-final-readiness/runtime-integration/doc.go` — public-mode package
  documentation.
- `plans/rc1-final-readiness/runtime-integration/README.md` — public run
  instructions plus historical development section.
- `plans/rc1-final-readiness/t09-published-integration.md` — this report.
- `plans/rc1-final-readiness/evidence/t09-published-integration-*.txt` — new
  proof logs.
- `plans/rc1-final-readiness/evidence/t09-development-*` — preserved
  pre-conversion snapshots.

No `pkg/`, root or shared manifest was changed by this task.

## Reviewer checklist

1. Confirm `go.mod` has `"Replace": null`, no `go.work`, and only the five
   public Andurel versions above.
2. Confirm the origin commits match the local tags (`718076ed...` for anduin
   `v0.8.0`, `c6c724fa...` for inertia `v0.14.1`) and that the published sums
   match `go.sum` and the evidence, not hand-written values.
3. Confirm the tag archives match the authenticated module-cache trees.
4. Re-run `gofmt`, `go fix`, `go vet`, the readonly/full/10x tests, Zig race,
   `go mod verify` and `go mod tidy -diff`.
5. Confirm the strict authenticated bag/provider and themed document/X-Inertia
   global/host paths, 409/303/403 cookie persistence, JSON consumers,
   nested/retry scopes and resource/cancel liveness all pass.
6. Confirm historical `t02` reports/evidence and the `t09-development-*`
   snapshots are present and unchanged.
7. Confirm no root/shared/`pkg` manifest changed and no commit, tag, push or
   publication occurred.

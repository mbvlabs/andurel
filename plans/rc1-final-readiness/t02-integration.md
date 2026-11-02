# T02 runtime integration (development harness)

Status: **READY FOR INDEPENDENT REVIEW.** Development-only cross-module test
harness. No commit, tag, publish, version, pin, manifest or application change.
No file under `pkg/`, the root module or the workspace manifests was modified by
this task. This is **not** release proof and does not claim T02/T09 acceptance.

## Identity and environment

Recorded from the shared rc.1 worktree while T01–T04 work was in flight:

```text
Framework: /home/mbv/work/andurel-rc1-work
HEAD:      289b1778de20f86210dbec749a1c622ff6018d65
Branch:    fix/rc1-final-readiness
Go:        go version go1.27.3 linux/amd64
GOWORK:    off (explicit; the repository go.work is not used)
Module:    andurel.local/rc1/runtime-integration
Location:  plans/rc1-final-readiness/runtime-integration/
```

The worktree is intentionally dirty from parallel rc.1 workers. Relevant T02
source under test is the coordinated `pkg/anduin` error lifecycle and the
implemented `pkg/inertia` capture coordination; `pkg/kiks` runtime code is
unchanged. The exact local source identity is the working tree, not a commit.

`go.mod` has explicit local replacements and no workspace dependency:

| Module | Graph version | Replacement |
| --- | --- | --- |
| `github.com/mbvlabs/andurel/pkg/anduin` | v0.6.2 | `../../../pkg/anduin` |
| `github.com/mbvlabs/andurel/pkg/inertia` | v0.0.0 (local) | `../../../pkg/inertia` |
| `github.com/mbvlabs/andurel/pkg/kiks` | v0.0.0 (local) | `../../../pkg/kiks` |
| `github.com/mbvlabs/andurel/pkg/routing` | v0.9.0 | `../../../pkg/routing` |
| `github.com/mbvlabs/andurel/pkg/telemetry` | v0.2.3 | `../../../pkg/telemetry` |

`routing` is required by the real router host registry. `telemetry` is an
anduin compile-time dependency; replacing it locally keeps the harness fully
source-local. The graph versions shown are requirements inherited from the
local module manifests; `replace` is what is actually built. `templ` is pinned
to `v0.3.1020`, matching the local modules.

All tests use `httptest` only with synthetic hosts, synthetic session/cookie
values and a test renderer root. There is no real login data, database, network,
Docker, production service or application process.

## Coverage

All tests are default-on; there is no build tag, skip or environment variable.
The suite has 11 test functions with document / X-Inertia and global / host
variants.

| Test | Pins |
| --- | --- |
| `TestHostAndGlobalErrorsKeepKiksContext` | Global and host guards, document and X-Inertia. The shared provider is strict: `kiks.Get[*sessionState]` must not return `ErrNoBag`. Asserts the ErrorHandler sees an authenticated session and that both document and X-Inertia responses are the themed 403 page with `auth.isAuthenticated=true`. |
| `TestEnclosingMiddlewareResourcesAreLiveDuringErrorHandler` | Global and host resource/cancel middleware enclosing the failing guard, document and X-Inertia. ErrorHandler sees the resource value, `ctx.Err()==nil` and `context.Cause==nil` while rendering; cleanup runs exactly once afterwards with the synthetic cancellation cause. |
| `TestDelayedRouteErrorUsesLiveRequestCause` | A route-scope delayed render takes `Err`/`Cause` from the live request even when the request context was canceled with a custom cause before dispatch. |
| `TestErrorRendererBagMutationsPersistWithFinalStatus` | ErrorHandler mutates the real kiks bag and writes 409 or 303, document and X-Inertia. One session save; the dirty session and flash survive a follow-up read. The 403 control keeps the dirty session but emits a cleared flash cookie, and the follow-up sees no flash. |
| `TestErrorRendererFlashStaysOutOfTheResponseBody` | A flash is a cookie concern; it does not appear in the themed body. |
| `TestInertiaCaptureDeliversRenderedErrorExactlyOnce` | The final response sink records exactly one header write and one body write for global/host and document/X-Inertia error pages; the component appears once, never the discarded success component. |
| `TestBufferedSuccessIsReplacedByOuterHostError` | An Inertia success page is buffered, an outer host middleware then fails, and the error page replaces it. No stale success body or success-only header is delivered; the base header survives; the final kiks session is saved once with the 403. |
| `TestRouteAPIErrorConsumerReturnsOneJSONResponse` | An `APIDocumentationErrors`-style route consumer returns one JSON 401 or 422 and nil. The router ErrorHandler is never called, no themed page/root is rendered, and the delivery has one header and body write. |
| `TestNestedCatchErrorsConsumeAndRetry` | A host `CatchErrors` observer defers to an enclosing global consumer; a host `CatchErrors` scope retries once and lets the success page commit. Bag mutations persist on the consumer's final status. |
| `TestRouteErrorDefersToEnclosingCatchErrors` | A route handler error is carried out of ServeMux through request state and consumed by an enclosing global or host `CatchErrors` scope, with one JSON 422 and no router render. |
| `TestAuthOnlyStyle303RedirectKeepsKiksCookies` | The AuthOnly-style host middleware destroys the session, stores an encrypted return target and returns a 303 without an error. The destroyed session cookie and return-target cookie persist for document and X-Inertia requests; the follow-up reads the expected admin path. |

## Red evidence

Two separate red records are kept; neither is a current passing proof.

1. **Published-pin compile red.** A disposable copy of this suite under
   `/tmp/opencode/t02-integration-red` used published
   `anduin v0.7.1`, `inertia v0.14.0`, `kiks v0.6.1`, `routing v0.9.1` and
   `GOWORK=off`:

   ```text
   ./api_consumer_test.go:156:19: undefined: anduin.CatchErrors
   ./api_consumer_test.go:182:22: undefined: anduin.CatchErrors
   ./api_consumer_test.go:284:22: undefined: anduin.CatchErrors
   FAIL	andurel.local/rc1/runtime-integration-red [build failed]
   ```

   This is expected: the suite targets the in-flight coordinated API. It is not
   behavioral proof and must not be cited as a published-dependency run.

2. **Historical B7-005 behavior red.** The read-only beta.7 record
   `/home/mbv/work/andurel-site/plans/beta7-port/evidence/context-repro/`
   shows host/global middleware errors returning plain `Forbidden` with
   `kiks: no cookie bag on context` and `marker_seen=false` under the published
   pre-fix stack. Those assertions characterize the old bug; the current suite
   asserts the desired fixed behavior instead. The historical reproduction was
   not rerun or modified here.

Full red note: `plans/rc1-final-readiness/evidence/t02-integration-red.txt`.

No current runtime defect was found by the combined suite. If a future run
fails, report the exact failing test (`GOWORK=off go test -run <name> -v`) to
the module owner rather than patching `pkg/anduin`, `pkg/inertia` or `pkg/kiks`
from this harness.

## Green verification

Run from `plans/rc1-final-readiness/runtime-integration` with `GOWORK=off`:

```sh
gofmt -l .                 # no output
GOWORK=off go fix ./...    # no source changes
GOWORK=off go vet ./...    # exit 0
GOWORK=off go test -count=1 ./...   # ok
GOWORK=off go test -count=10 ./...  # ok (repeat)
GOWORK=off CGO_ENABLED=1 \
  CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' \
  go test -race -count=1 ./...     # ok
```

Logs:

- `plans/rc1-final-readiness/evidence/t02-integration-baseline.txt`
  (HEAD, status, Go, `GOWORK=off`, resolved local replacements)
- `plans/rc1-final-readiness/evidence/t02-integration-green.txt`
  (gofmt / go fix / vet / test / repeat)
- `plans/rc1-final-readiness/evidence/t02-integration-race.txt`
  (Zig cgo race)
- `plans/rc1-final-readiness/evidence/t02-integration-red.txt`
  (published compile red and historical behavior reference)

`git diff --name-only -- go.mod go.sum go.work go.work.sum pkg/*/go.mod
pkg/*/go.sum` is empty: this task changed no root, shared, workspace or package
module manifest.

## Local-replace limits

- These tests exercise the current local source only. They do **not** prove
  that published modules compile, resolve through a public proxy/checksum, or
  contain the coordinated API. `anduin v0.6.2` / `routing v0.9.0` appear only as
  replaced graph requirements inherited from local manifests.
- No release-shaped proof is provided: no fresh scaffold, upgrade path,
  release goldens, public-pin checker, site acceptance or T07 live/image run.
- The suite deliberately depends on the in-flight API and is compile-red against
  published pins. T08/T09 must re-run the positive behavior with owner-published
  modules and no local replacements.
- Tests use only `httptest`; they do not validate a real listener, TLS/proxy
  behavior, database session stores, Docker, deployment or production data.
- The telemetry and templ choices above are harness convenience, not release
  pins.

## Files

- `plans/rc1-final-readiness/runtime-integration/go.mod`,
  `go.sum` — standalone development module with local replaces.
- `plans/rc1-final-readiness/runtime-integration/doc.go`,
  `README.md` — development-only declaration and run instructions.
- `plans/rc1-final-readiness/runtime-integration/fixtures_test.go` — real
  router/kiks/renderer fixture, counting store, strict provider, request and
  delivery helpers.
- `host_global_error_test.go`, `resource_test.go`, `persistence_test.go`,
  `capture_test.go`, `buffered_success_test.go`, `api_consumer_test.go`,
  `authonly_test.go` — the default-on cases above.
- `plans/rc1-final-readiness/evidence/t02-integration-*.txt` — red/green logs.
- This report.

## Reviewer checklist

- Confirm the strict provider is reached with the kiks bag for both global and
  host errors, on document and X-Inertia visits, and sees an authenticated
  session.
- Confirm resource/cancellation liveness and cleanup ordering, including the
  custom live `Cause` test.
- Confirm exactly-one session save for 409/303/403, flash persistence and 403
  flash clearing through follow-up reads.
- Confirm one header/body delivery for capture and no stale success body/header
  after a buffered success is replaced by an outer host error.
- Confirm route API consumers render one JSON 401/422 with zero ErrorHandler
  calls, and that nested/outer `CatchErrors` consume/retry route and host errors.
- Confirm AuthOnly-style 303 cookies survive, and re-run the green and race
  commands with `GOWORK=off`.

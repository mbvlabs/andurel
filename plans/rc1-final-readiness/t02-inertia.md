# T02 — Inertia capture coordination

Status: **READY FOR RE-REVIEW (review P2 fixes included).** Development change
only. This note does not publish a module, change a version/pin/manifest, or
claim published dependency or release acceptance. kiks integration and
published-module proof remain T08/T09 work.

## Baseline and scope

Recorded from the shared rc.1 worktree:

```text
HEAD:     289b1778de20f86210dbec749a1c622ff6018d65
branch:   fix/rc1-final-readiness
go:       go version go1.27.3 linux/amd64
GOWORK:   /home/mbv/work/andurel-rc1-work/go.work
GOFLAGS:  (empty)
CGO:      0 by default (race run below uses the Zig cc shim)
module:   github.com/mbvlabs/andurel/pkg/inertia
```

The worktree is shared and already contains unrelated changes. This task changed
only:

- `pkg/inertia/middleware.go`
- `pkg/inertia/middleware_test.go` (new)
- `pkg/inertia/CHANGELOG.md`
- this plan and its evidence files

`pkg/inertia/go.mod`, `go.sum`, versions, pins and generated artifacts are
unchanged. The module still pins published `anduin v0.6.2`.

Full metadata: `evidence/t02-inertia-baseline.txt`.

## Implemented contract

`pkg/inertia/middleware.go` now coordinates with the new anduin error lifecycle:

1. `captureWriter` implements `anduin.ResponseBuffer`.
2. `DiscardBuffered` resets the buffered status and body. It restores response
   headers from a snapshot taken when capture began, then re-applies response
   metadata that belongs to the request rather than the discarded success body:
   `Allow`, `Vary`, `Retry-After`, `WWW-Authenticate`, `Proxy-Authenticate`,
   and security/CORS policy headers (`Content-Security-Policy*`,
   `Access-Control-*`, `Cross-Origin-*`, `X-Frame-Options`,
   `X-Content-Type-Options`, HSTS, referrer/permissions policy, and related
   headers). Stale success headers such as `Content-Type`, `Location`,
   `X-Inertia` and handler-specific headers are dropped. It is safe to call more
   than once.
3. After `next` returns:
   - `err == nil`: the existing success path runs unchanged through the new
     factored `finish` method (normalize, redirect reflash, commit). A reflash
     failure still returns without committing on the success path, preserving
     the prior router-error behavior.
   - `err != nil` and `anduin.RenderedError(ctx) != nil`: the router already
     rendered an error into this capture. `finish` normalizes, reflashes and
     commits it. The original error is returned unchanged when finish succeeds;
     a normalization/reflash/commit failure is returned with
     `errors.Join(originalErr, finishErr)`, preserving `errors.Is`/`errors.As`
     for both. If a rendered redirect's reflash fails, the normalized response
     is still committed: the router has already marked the response committed
     and trusts this buffer, so an empty implicit 200 is not acceptable. The
     reflash error (and any commit error) is returned alongside the original.
   - `err != nil` and `anduin.RenderedError(ctx) == nil`: the capture is
     discarded as before. This is the pending `anduin.CatchErrors` scope case:
     the delayed scope renders after this middleware unwinds, so Inertia must
     not commit its stale capture or consume the error.

The 405 `Allow` metadata is also captured by the current shared anduin router
around `DiscardBuffered`; this task did not edit anduin. Inertia additionally
classifies `Allow` as response metadata so its own buffer reset cannot erase it.
No general arbitrary-metadata ownership API was added; that would require
anduin/parent coordination.

The success path is factored into `captureWriter.finish` without changing its
normalization/reflash/commit order. The `wasRedirect` decision is still taken
before normalization, matching the previous success behavior.

## Red before implementation

Command:

```sh
cd pkg/inertia
go test -run 'TestMiddleware' -count=1 -v .
```

Before the middleware change, the focused suite failed on the coordinated
cases: host/global guard Inertia visits and the middleware-context provider were
discarded by the old error drop, the router's old-capture safety fallback
produced plain 500 instead of the themed commit, rendered redirects were not
normalized/reflashed, a buffered success could not be replaced by the error
page, and a commit failure was never observed. The document controls and the
pending CatchErrors discard control passed.

Failure excerpt:

```text
--- FAIL: TestMiddlewareCommitsRenderedGuardError/global/inertia_visit
    status = 500, want 403: Internal Server Error
--- FAIL: TestMiddlewareCommitsRenderedGuardError/host/inertia_visit
    status = 500, want 403: Internal Server Error
--- FAIL: TestMiddlewareErrorPageUsesMiddlewareContextProvider
    status = 500, want 403: Internal Server Error
--- FAIL: TestMiddlewareNormalizesRenderedRedirectAndReflashes
    status = 500, want 409: Internal Server Error
--- FAIL: TestMiddlewareDiscardsBufferedSuccessBeforeHostError
    status = 500, want 403: Internal Server Error
--- FAIL: TestMiddlewareCommitErrorPreservesOriginalError
    returned error = ... want the commit error
--- PASS: TestMiddlewareDiscardsCaptureWhileCatchErrorsIsPending
```

Full log: `evidence/t02-inertia-red.txt`.

### Review P2 red

Command:

```sh
cd pkg/inertia
go test -run 'TestMiddlewarePreservesAllowOnInertiaMethodNotAllowed|TestMiddlewarePreservesDownstreamSecurityAndCORSHeaders|TestMiddlewareRenderedRedirectReflashFailureDeliversResponse|TestMiddlewareDiscardsBufferedSuccessBeforeHostError' -count=1 -v .
```

Before the P2 fixes, downstream CSP/CORS headers set after capture were stripped
by the blanket snapshot, the buffered-success case lost its security headers,
and a rendered redirect whose reflash failed delivered an empty implicit 200 with
redirect headers. The concrete failure excerpt:

```text
--- FAIL: TestMiddlewarePreservesDownstreamSecurityAndCORSHeaders
    Content-Security-Policy = "", want "default-src 'self'"
--- FAIL: TestMiddlewareRenderedRedirectReflashFailureDeliversResponse
    status = 200, want 409
--- FAIL: TestMiddlewareDiscardsBufferedSuccessBeforeHostError
    ErrorHandler saw X-Content-Type-Options = "", want nosniff
```

`TestMiddlewarePreservesAllowOnInertiaMethodNotAllowed` passed in this shared
worktree because the parallel anduin change already captures and restores the
ServeMux `Allow` around `DiscardBuffered`; the test remains the required
end-to-end guard, and the capture-level metadata test now also covers `Allow`.
Full log: `evidence/t02-inertia-review2-red.txt`.

### Review P2 follow-up red (standard error metadata)

Command:

```sh
cd pkg/inertia
go test -run 'TestCaptureWriterDiscardBufferedResetsResponseAndPreservesMetadataHeaders|TestMiddlewarePreservesRetryAfterFromRateLimit|TestMiddlewarePreservesAuthenticationChallenge' -count=1 -v .
```

Before the follow-up fix, the metadata allowlist omitted `Retry-After`,
`WWW-Authenticate` and `Proxy-Authenticate`, so a captured and discarded error
response lost its throttle/auth challenge headers:

```text
--- FAIL: TestCaptureWriterDiscardBufferedResetsResponseAndPreservesMetadataHeaders
    Retry-After = "", want 60
--- FAIL: TestMiddlewarePreservesRetryAfterFromRateLimit
    Retry-After = "", want 60
--- FAIL: TestMiddlewarePreservesAuthenticationChallenge/inertia_visit
    WWW-Authenticate = ""
```

The document authentication control passed because documents bypass capture.
Full log: `evidence/t02-inertia-review3-red.txt`.

## Focused test coverage

`pkg/inertia/middleware_test.go` drives the behavior through
`anduin.Router`; there is no kiks dependency in this module.

| Test | Covers |
| --- | --- |
| `TestCaptureWriterDiscardBufferedResetsResponseAndPreservesMetadataHeaders` | status/body reset, stale success headers removed, base headers plus `Allow`, `Vary`, security, `Retry-After`, `WWW-Authenticate` and `Proxy-Authenticate` metadata preserved, idempotent discard, only error page delivered |
| `TestMiddlewareCommitsRenderedGuardError` | global and host guard errors on document and Inertia visits; themed status/body, `Vary: X-Inertia`, `X-Inertia` JSON header, exactly one ErrorHandler call, original returned error identity |
| `TestMiddlewarePreservesAllowOnInertiaMethodNotAllowed` | real `anduin.Router` POST-to-GET-only Inertia visit: themed 405 delivered, `Allow: GET, HEAD`, one render |
| `TestMiddlewarePreservesRetryAfterFromRateLimit` | real `anduin.RateLimit(..., nil)` route middleware: over-limit Inertia visit keeps `Retry-After: 60` on the themed 429, one render |
| `TestMiddlewarePreservesAuthenticationChallenge` | 401 authentication challenge keeps `WWW-Authenticate` for Inertia visits, with a document control |
| `TestMiddlewarePreservesDownstreamSecurityAndCORSHeaders` | headers installed by middleware after capture survive a rendered guard error: CSP, framing, CORS and `Vary: Origin`, while no stale success headers leak |
| `TestMiddlewareErrorPageUsesMiddlewareContextProvider` | error page shared provider sees middleware-supplied context while the rendered page is committed |
| `TestMiddlewareNormalizesRenderedRedirectAndReflashes` | router-rendered redirect is converted to the fragment 409 control response, reflash runs once, no body, `Vary` retained |
| `TestMiddlewareRenderedRedirectReflashFailureDeliversResponse` | rendered redirect with failing reflash still commits the 409 control response; original and reflash errors are both visible via `errors.Is`, one ErrorHandler call |
| `TestMiddlewareDiscardsCaptureWhileCatchErrorsIsPending` | outer `CatchErrors` leaves `RenderedError` nil; stale success capture is discarded and the delayed scope renders exactly once |
| `TestMiddlewareDiscardsBufferedSuccessBeforeHostError` | buffered success is replaced by the host error page; stale success header dropped, base and security/CORS headers kept, only error component in body, one render |
| `TestMiddlewareCommitErrorPreservesOriginalError` | commit write failure is joined with the original error; `errors.Is` matches both, one render, intended status attempted |

## Verification

Workspace-local development run. Logs are under `plans/rc1-final-readiness/evidence/`:

```sh
cd pkg/inertia
gofmt -l .                 # no output
go fix ./...               # no source changes
go vet ./...               # exit 0
go test -count=1 ./...     # ok github.com/mbvlabs/andurel/pkg/inertia
CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' \
  go test -race -count=1 ./...
                           # ok github.com/mbvlabs/andurel/pkg/inertia
go test -run 'TestMiddleware|TestCaptureWriter' -count=10 .
                           # ok (repeat run)
git diff --check           # exit 0
```

Logs: `evidence/t02-inertia-validation.txt`,
`evidence/t02-inertia-race.txt`.

### Published/GOWORK=off limitation

This is **not** published-dependency proof. With `GOWORK=off`, the module still
resolves published `anduin v0.6.2`, which does not contain `RenderedError`,
`ResponseBuffer`, `CatchErrors` or the host APIs used by the coordinated tests.
The expected development failure is recorded in
`evidence/t02-inertia-gowork-off.txt`:

```sh
cd pkg/inertia
GOWORK=off go test -count=1 ./...
# undefined: anduin.RenderedError
# undefined: anduin.ResponseBuffer
# undefined: anduin.CatchErrors
# Host.Isolate / Host.Use undefined
# FAIL (build failed)
```

A green `GOWORK=off` run is only possible after the anduin owner publishes the
new API and this module's pin is updated in T08. Do not cite the workspace run
as release-shaped proof.

## Known limitations / follow-ups

1. **Published pin gate.** `pkg/inertia` cannot consume the new anduin API with
   `GOWORK=off` until the owner-published version is available and pinned.
2. **No kiks dependency.** Kiks bag persistence ordering is covered by the
   coordinated anduin fake-bag tests; published kiks/inertia integration belongs
   to T09.
3. **Pending delayed scopes keep the old discard behavior.** An outer
   `CatchErrors` renders after Inertia unwinds. The capture's buffered body is
   not committed; headers already written to the underlying writer by the
   discarded attempt are not rolled back on this path, matching the prior
   behavior.
4. **Header rollback is snapshot plus a narrow metadata policy.** Headers set
   before capture starts survive. After the base snapshot is restored, only the
   listed metadata class is re-applied from the discarded response: router
   `Allow`, cache `Vary`, security/CORS policy, and standards-defined
   `Retry-After`/`WWW-Authenticate`/`Proxy-Authenticate` error metadata. This is
   intentionally not a general "keep all headers" list, so arbitrary success
   headers stay discarded. It is not a response-header transaction and does not
   cover writers outside the Unwrap chain. Arbitrary caller metadata would need
   an explicit ownership API in anduin; none was added here.
5. **Reflash policy is rendered-error specific.** A router-rendered redirect
   commits its normalized response even when reflash fails because the router
   has already marked it committed and trusts the buffer. The success path keeps
   its prior no-commit behavior on reflash failure so the router can render the
   failure. Both paths return the original/reflash/commit errors without a
   second ErrorHandler render.
6. **No commit/pin/publish.** No module version, tag, registry artifact, release
   note, go.mod/go.sum entry or pin was changed.

## Reviewer checklist

- Confirm success behavior is unchanged: `finish` still computes
  `wasRedirect` before normalization, reflashes on either 3xx condition and
  commits once. On success-path reflash failure it still does not commit, as
  before.
- Confirm a rendered redirect whose reflash fails commits the normalized
  response, returns the original and reflash errors together, and does not
  invoke the ErrorHandler again.
- Confirm `RenderedError` alone gates the error commit; a pending
  `CatchErrors` error still discards the capture and returns unchanged.
- Confirm a rendered error returns the original error object when finish
  succeeds and uses `errors.Join` only when finish fails.
- Confirm `DiscardBuffered` resets status/body, restores the base snapshot and
  re-applies `Allow`, `Vary`, security/CORS, `Retry-After` and authentication
  challenge metadata while dropping stale success body/header state.
- Confirm the actual Router Inertia 405 case retains `Allow: GET, HEAD`, the
  real RateLimit case retains `Retry-After`, and the authentication challenge
  retains `WWW-Authenticate` on Inertia visits.
- Confirm the downstream security/CORS test retains headers installed after
  capture.
- Confirm every coordinated test asserts at most one `ErrorHandler` call.
- Confirm `GOWORK=off` failure is documented as expected until anduin is
  published; do not treat workspace evidence as published proof.

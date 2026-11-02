# T02 coordinated runtime implementation (anduin)

Status: **IMPLEMENTED IN `pkg/anduin`; INDEPENDENT REVIEW REQUIRED.** This is a
staged development change on the shared rc.1 worktree. It does not publish a
module, change a pin/manifest, or claim T02/T09 acceptance. Inertia capture
support consumes the new interface and remains a separate worker. Published
kiks/inertia integration and release-shaped proof remain T08/T09.

Owner decision: option B, coordinated runtime scope (`t02-design-decision.md`).
The internal design choices below were delegated to engineering judgement.

## Contract

### Host and global errors: early, live render

Every global and host middleware layer is boundary-wrapped. Outside a delayed
scope, an error is rendered at the failing boundary before enclosing middleware
unwinds:

- `ErrorHandler` receives the context actually passed to the failing layer, so
  middleware values (for example the kiks bag) are present.
- Enclosing middleware resources and cancellation are live. A middleware that
  itself returns after its own defers have run cannot offer its own resources;
  the guarantee is only for middleware still enclosing the failing boundary.
- Response writers are still installed in their normal order. Kiks-style
  `persistWriter` observes the error status and saves before its middleware
  returns. The accepted 409/303 behavior and 403 clear control are tested with a
  local fake that mirrors kiks.
- Middleware that needs to keep observing, consuming, translating or retrying
  an error wraps its layer with `CatchErrors`. Without it, an inner guard has
  already rendered before an outer recovery middleware sees the error. This is
  the explicit compatibility change from beta.7.

### Route and handler errors: implicit delayed scope

Route middleware plus handler run in an implicit delayed scope. The error is
rendered after route middleware unwinds, preserving existing route consumers
(`APIDocumentationErrors` returns its JSON and returns nil; no router render).
The scope records the deepest boundary values of the latest failing pass:

- repeated `next` calls may fail/fail or fail/success; fail/fail renders the
  latest pass, fail/success renders nothing;
- a middleware may translate the final error; the translated error is rendered
  with the deepest recorded values;
- route middleware consuming with nil prevents the render;
- route middleware may observe/wrap the original error; host/global middleware
  still observe nil for consumed route errors, as before.

Route-scope middleware resources are **not** claimed live: the route scope
renders after route middleware returns, so its own defers have run. The
`ErrorHandler` still sees the scope's values. Cancellation/deadline/cause come
from the live request context, not from a canceled child retained as a value.

### `CatchErrors` explicit delayed scope

`CatchErrors(middleware)` delays rendering of everything inside that middleware
until it returns. It is the additive API for global/host consumers and retries:

- propagates the original error unchanged;
- returns nil only if the wrapped middleware consumes the error;
- renders once at scope exit with the latest failing pass's deepest values;
- merges values from the recorded context with the live request context;
- never invokes the ErrorHandler when the scope's final error is nil;
- a route consumer keeps the implicit scope; a host/global consumer should use
  this wrapper.

### Once-render guard, retries, committed errors

`requestState` records `rendered` **before** invoking the ErrorHandler. A
reentrant boundary cannot render again. After a render:

- `ErrorHandler` is never called a second time for the same request;
- a retry into the handler chain returns `*CommittedError` matching
  `ErrResponseCommitted`; it never runs the inner handler again;
- `CommittedError.Unwrap` exposes the rendered error, so `errors.Is`/`errors.As`
  still see it;
- `RenderedError(ctx)` reports the rendered error, used by coordinated capture
  middleware to decide to commit the captured page.

### Nested delayed scopes

Delayed scopes form a stack. When an inner route/`CatchErrors` scope exits with
an error and an outer delayed scope is still active, the inner scope does not
render: it propagates its deepest failing attribution into the outer scope. The
outer owner may consume, translate or retry before the single render. A route
error under an enclosing host/global scope is carried out of ServeMux through
request state so the enclosing observer sees it; without an enclosing scope the
existing route-consuming behavior is unchanged.

### Context merge

Delayed scopes use `context.WithoutCancel(values)` as the value source, so a
canceled child context cannot leak cancellation through a value key. A bridge
`context.WithCancelCause` is created over those values; a live cancellation
watcher propagates only live cancellation (and its cause) into the bridge, and an
already-canceled live context is applied synchronously. The wrapper embeds the
bridge for `Value`, overrides `Deadline` with the live deadline, and uses a
separate wrapper-owned `Done` plus a first-classification freeze. The winning
cancellation path stores the intended `Err` (`DeadlineExceeded` for an expired
deadline, `Canceled` for explicit cancellation or cleanup), cancels the standard
bridge with its cause, and closes the wrapper `Done` last. `Err` waits for that
wrapper `Done` once the bridge is canceled, so it never returns nil while
`Cause` is already non-nil, then returns the frozen classification; `Cause`
continues to come from the standard bridge, including custom deadline causes.
The distinct wrapper `Done` also prevents standard child derivation from
discovering the bridge's internal cancel state, so
`WithCancel`/`WithCancelCause` children use public propagation and inherit the
frozen classification. No deadline timer is recreated; deep nil value masks
remain stable, and cleanup stops the watcher and closes the wrapper context
after the synchronous render. Values a middleware creates but never passes to
`next` remain unrecoverable, as before.

### Response buffers and delivery

- `ResponseBuffer { DiscardBuffered() }` is the coordinated capture interface.
  Before any error render, the router walks the response writer `Unwrap` chain
  and calls `DiscardBuffered` on every buffer, so a buffered successful page is
  replaced by the error page rather than concatenated. Implementations should
  reset the buffered status, body and response-owned headers.
- The router wraps the original writer in a delivery tracker. `ResponseBuffer`
  implementations are trusted to deliver the rendered error; `RenderedError`
  gives them the signal to commit.
- Old captures without `ResponseBuffer` are detected as mixed
  new-anduin/old-inertia: if the render wrote only into the capture and no bytes
  reached the real writer, the router logs once and writes **one** safe plain
  500 directly to the request's root writer. It never invokes `ErrorHandler`
  again. This is a safety fallback, not themed error support.
- The router preserves router-owned error metadata across capture discards: the
  ServeMux `Allow` header for a 405 is snapshotted before `DiscardBuffered` and
  restored after, including the fallback discard. This keeps `Allow` independent
  of a buffer's header snapshot; CORS/security headers remain the responsibility
  of the coordinated inertia header handling.
- If some bytes were already delivered before the discarded render, the router
  logs the discard but cannot replace the response; no second render happens.
- Writer semantics remain based on `ResponseWriter(ctx)`. The delivery wrapper
  implements/forwards `Flush`, `FlushError`, `Hijack`, `Push` and `Unwrap` so
  `http.ResponseController` and existing optional-interface checks keep working.
  It does not replace per-middleware writers in the request state.
- Renderer errors and panics are recovered, logged once with the final error,
  and followed by one plain fallback. Any partial error page already written
  into a cooperative capture is discarded before that fallback, so the capture
  holds only the plain 500 rather than a partial page and its status.
- A failed plain fallback does not recurse.
- `FlushError` is forwarded through the renderer write wrapper as well as the
  delivery wrapper, so `http.ResponseController` inside an `ErrorHandler`
  cannot report a successful flush when the underlying writer rejected it.

### Mux and B7-007

ServeMux 404/405 are returned through the host boundary instead of rendered
directly inside dispatch. `Allow` still survives because ServeMux sets it before
`WriteHeader`. The mux error is rendered once with host values; a `CatchErrors`
host scope can consume/translate it. Fallback-to-primary still bypasses host
middleware; host middleware still runs before an Isolate host's own 404/405 and
can redirect first.

## API availability/changes (additive)

New in `pkg/anduin/errorlifecycle.go`:

```go
func CatchErrors(middleware Middleware) Middleware
func RenderedError(ctx context.Context) error
type ResponseBuffer interface { DiscardBuffered() }
var ErrResponseCommitted error
type CommittedError struct { Err error }
// Error(), Unwrap() error, Is(target error) bool
```

New in `pkg/anduin/delivery_writer.go` (unexported):

```go
type deliveryTracker
type deliveryWriter // Unwrap, Flush, FlushError, Hijack, Push
```

Changed internal behavior/API comments:

- `applyMiddleware` still builds outer-to-inner; the boundary hook now renders
  or records per scope.
- `Router.dispatch` returns mux errors through the chain.
- `ErrorHandler`, `Middleware`, package docs and `CHANGELOG.md` describe early
  host/global rendering and `CatchErrors`.
- No manifest, module version, pin, generated artifact or publication change.

## Reviewer rereview fixes (2026-11-02)

The expanded implementation was re-reviewed and held on five items. All are
addressed in the same `pkg/anduin` surface:

1. Nested delayed scopes now propagate the inner failing attribution to the
   enclosing active scope instead of rendering. An enclosing `CatchErrors`
   owner can consume, translate or retry first; route errors travel out of
   ServeMux through request state when such an outer scope exists. Tests cover
   inner consume, translation, inner retry, outer retry and route propagation
   with latest-pass attribution.
2. Post-render logging compares errors only when their actual dynamic values are
   reflect-comparable. A comparable type holding a slice is not compared; named
   slices and other non-comparable values are conservatively logged rather than
   equated by message. Distinct comparable types or wrappers with the same text
   log as changed.
3. Delayed contexts now use a `WithCancelCause` bridge over the detached values,
   freeze the first classification through a wrapper-owned `Done`, and close that
   `Done` last after canceling the bridge with its cause. `Deadline` reports the
   live deadline; no timer is recreated. An expired or firing
   `WithDeadlineCause` reports `context.DeadlineExceeded` with its custom cause;
   cleanup reports `Canceled`, cleanup-before-live-deadline stays frozen,
   `Err` waits for wrapper-Done publication after bridge cancellation, and
   `WithCancel`/`WithCancelCause` children inherit the same classification.
   Tests include already-expired, firing, cleanup races, publication wait, deep
   nil masks, concurrent Cause reads and root cancellation.
4. A cooperative capture is discarded again when the ErrorHandler has already
   written a partial error page and then returns an error or panics. The single
   fallback is always plain 500, so the capture holds only the fallback rather
   than a partial page and status.
5. `renderWriteWriter` forwards `FlushError` (and keeps Flusher/Hijacker/Push/
   Unwrap parity), so `http.ResponseController.Flush` inside an ErrorHandler
   reports the underlying flush failure instead of success.
6. The ServeMux `Allow` header for a generated 405 is recorded before the mux
   error leaves dispatch and restored after every capture discard, so an
   inertia header snapshot cannot strip it. Broader CORS/security header and
   reflash delivery fixes remain the inertia owner's coordinated work.

Red evidence before these fixes:
`plans/rc1-final-readiness/evidence/t02-coordinated-rereview-red.txt`.
This third rereview (actual-value comparison and bridge context) has red
evidence in `plans/rc1-final-readiness/evidence/t02-coordinated-rereview3-red.txt`.
The narrow `Err` deadline-classification follow-up has red evidence in
`plans/rc1-final-readiness/evidence/t02-coordinated-rereview4-red.txt`.
The frozen-classification/derived-child refinement has red evidence in
`plans/rc1-final-readiness/evidence/t02-coordinated-rereview5-red.txt`.
The publication-window `Err` wait has red evidence in
`plans/rc1-final-readiness/evidence/t02-coordinated-rereview6-red.txt`.

## Tests (default-on)

The three prior `TestDesignHold` skips were removed; there is no opt-in
environment variable. These tests run on every module test invocation.

`pkg/anduin/router_error_context_test.go`:

- `TestErrorHandlerReceivesLiveContextAndActiveResources` — global and host:
  live `ctx.Err`, live resource, render before cleanup, original error identity.
- `TestRouteScopeRetainsValuesButResourcesAreUnwound` — deepest route value
  reaches the handler; canceled child does not leak; route resource is already
  closed and documented.
- `TestErrorRenderSeesFinalBagStatus` — fake kiks bag: 409 save status plus
  flash, 303 save status, 403 clear control; render write happens before save.
- `TestRouteScopeRetryUsesLatestFailingPass` — fail/fail uses B; fail/success
  makes no ErrorHandler call.
- `TestHostUnscopedRetryIsBlockedAfterEarlyRender` — one early render for A;
  second `next` returns `ErrResponseCommitted`; guard is not re-entered.
- `TestErrorHandlerReceivesDeepestMiddlewareContext` — existing matrix for
  global/host/route/handler, isolated 404/405 with `Allow`, fallback success.
- `TestRouteErrorObservationAndUnwindContract`,
  `TestHostAndGlobalErrorObservationAndUnwindContract`,
  `TestSwallowedMiddlewareErrorIsNotRendered` (now uses `CatchErrors`),
  `TestErrorResponseCookieTiming`, B7-007 tests.

`pkg/anduin/router_error_lifecycle_test.go`:

- `TestCatchErrorsConsumesHostError` — explicit host consume, zero renders.
- `TestRouteMiddlewareConsumesErrorWithoutErrorHandler` — route/API consumer.
- `TestCatchErrorsAllowsHostRetry` — explicit retry scope, one render for B.
- `TestNestedCatchErrorsDefersToEnclosingScope` — nested host observer +
  global consumer: consume, translation, inner retry, outer retry and route
  propagation all defer to the enclosing scope; latest pass attribution is
  preserved and the inner scope never renders first.
- `TestPostRenderLoggingErrorComparison` — same comparable sentinel stays quiet;
  named slices and comparable structs with interface-held slices are
  conservatively logged without panic; distinct comparable types and
  same-message wrappers log as changed.
- `TestMergeErrorContextPreservesLiveSemantics` — expired/firing custom deadline
  causes report `DeadlineExceeded` with the custom Cause, live deadline, deep
  nil mask before/after cancellation, concurrent Cause reads, pre-Done Err nil,
  cleanup `Canceled`, cleanup-before-late-deadline frozen classification,
  cleanup-versus-deadline and cleanup-versus-cancellation race stability, root
  cancellation cause and no recreated deadline timer race.
- `TestMergedContextChildrenPreserveClassification` — `WithCancel` and
  `WithCancelCause` children see frozen `DeadlineExceeded`/custom cause for
  firing and already-expired deadlines, `Canceled` after cleanup-before-deadline,
  and stable classification under cleanup/deadline and cleanup/cancellation
  races.
- `TestMergedContextErrWaitsForFrozenPublication` — pauses between bridge
  cancellation and wrapper-Done close; `Err` blocks until publication and then
  returns the frozen error while `Cause` is the standard custom cause.
- `TestRouteScopeTranslationUsesDeepestValues` — translated final error with
  deepest values.
- `TestDelayedScopeMergesLiveCancellation` — deadline/value preservation and
  root cancellation cause.
- `TestResponseBufferDiscardsBufferedSuccessBeforeError` — stale buffered
  success replaced through an `Unwrap` chain, cooperative commit, one render.
- `TestLegacyCaptureDiscardFallsBackOnce` — old capture discards; one render,
  one log, one plain 500, no second ErrorHandler call.
- `TestJoinedErrorAfterRenderIsLoggedOnce` — a joined final error after an early
  render is logged once and does not trigger a second response.
- `TestFailedRendererDiscardsCooperativeBufferBeforePlainFallback` — a partial
  error page plus returned error or panic is discarded; the cooperative capture
  holds and delivers only the plain 500.
- `TestRendererWriterForwardsFlushError` — `http.ResponseController.Flush`
  inside the `ErrorHandler` returns the underlying `FlushError`, and the render
  wrapper exposes Flusher/FlushError/Hijacker/Unwrap.
- `TestMethodNotAllowedAllowSurvivesBufferDiscard` — router-owned `Allow`
  survives a capture whose `DiscardBuffered` resets headers.
- `TestErrorHandlerPanicFallsBackOnce` — panic recovery, one plain response.
- `TestRecoveredPanicRendersAtGlobalBoundary` — route/`CatchErrors` scopes are
  dropped while panicking, so an outer `Recover` renders at the live global
  boundary instead of leaving a stale delayed scope.
- `TestCommittedErrorUnwrapsOriginal` — sentinel plus original `errors.Is/As`.
- `TestResponseWriterSupportsFlushAndHijackThroughRouter` — real server,
  `http.ResponseController` flush and hijack through the delivery wrapper.

## Evidence

Baseline HEAD remains `289b1778de20f86210dbec749a1c622ff6018d65` plus the shared
worktree's parallel changes. T02 changed only `pkg/anduin/` and T02 plan files.

Red before implementation (tests referenced the not-yet-existing API):

```text
go test -run '<new T02 matrix>' -count=1 .
# undefined: CatchErrors, RenderedError, ErrResponseCommitted, CommittedError
```

Log: `plans/rc1-final-readiness/evidence/t02-coordinated-red.txt`.

Final module verification (exit 0):

```sh
cd pkg/anduin
gofmt -l .                 # no output
go fix ./...               # no source changes
go vet ./...               # exit 0
go test -count=1 ./...     # ok anduin, anduintest
GOWORK=off go test -count=1 ./...  # ok anduin, anduintest
CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' \
  go test -race -count=1 ./...     # ok anduin, anduintest
```

Logs: `evidence/t02-coordinated-validation.txt`,
`evidence/t02-coordinated-race.txt`.

Cross-module development smoke with the workspace-local anduin (not published
proof):

```sh
cd pkg/kiks && go test -count=1 ./...  # ok
```

`pkg/inertia` and the layout generated-config runtime smoke passed earlier in
this task, but the shared worktree's concurrent inertia-owner edits made that
package temporarily uncompilable at final T02 capture. That is separate-owner
work in progress, not a T02 pass or failure; see
`evidence/t02-coordinated-inertia-smoke.txt`. T09 published integration remains
the real cross-module gate.

Logs: `evidence/t02-coordinated-kiks-smoke.txt`,
`evidence/t02-coordinated-inertia-smoke.txt`.

## Known limitations / coordinated follow-ups

1. **Old published inertia capture.** Until the inertia worker implements
   `ResponseBuffer` and commits a page reported by `RenderedError`, an early
   host/global error render into the published capture is discarded. The anduin
   safety fallback sends a plain 500, not the themed page, and the capture's
   kiks writer does not persist the error render/status. Coordinated inertia
   pins and T09 published proof are mandatory before claiming host/global themed
   error support end to end.
2. **Consumer compatibility.** Global/host middleware that consumes errors must
   use `CatchErrors`. Route middleware keeps the implicit scope and needs no
   change. This is an intentional option-B compatibility change; it should be in
   rc.1 notes.
3. **Scope resource lifetime.** Early host/global renders guarantee enclosing
   middleware resources. Route/CatchErrors delayed renders guarantee values, not
   resources owned by middleware that has already returned. Error providers must
   not assume released transactions/connections are live.
4. **Kiks save placement.** Bag persistence must remain inside the kiks
   middleware's `next` return path, before it unwinds. No finalization hook is
   added, and wrapping kiks in an outer delayed scope would move rendering after
   its save. The fake bag tests pin the accepted ordering only; they are not
   published kiks proof.
5. **Unrecoverable values.** A context a middleware creates and never passes to
   `next` cannot be recovered. This is the existing API limit.
6. **Partial delivery.** If a response already delivered bytes, a discarded
   later render cannot be replaced; the router logs and avoids a double render.
7. **Mux observation change.** Host/global middleware now observe mux 404/405
   errors after the early render (they previously saw nil because dispatch
   rendered directly). Consumption/translation uses `CatchErrors`; the response
   contract and `Allow` header are unchanged.
8. **Capture header/reflash coordination.** `Allow` is preserved by the router
   independently of `DiscardBuffered`; broader CORS/security header restoration
   and reflash-on-rendered-redirect delivery remain the inertia worker's
   contract. No capture API change is made here until that worker proposes one;
   any such change must be additive and must not hide router error metadata.
9. **No publication or version.** No module version, pin, changelog cut or
   registry artifact is created. Published dependency proof remains T08/T09.

## Reviewer checklist

- Confirm early host/global render happens before enclosing middleware cleanup
  and that route scope keeps the delayed consumer contract.
- Confirm no path invokes `ErrorHandler` twice, including mixed old capture,
  renderer return error, and renderer panic.
- Confirm nested scopes propagate to the enclosing owner without an inner
  render, and that 405 `Allow` survives `DiscardBuffered`.
- Confirm retries after render return `ErrResponseCommitted` only through the
  committed wrapper and do not run the handler.
- Confirm `CatchErrors` consumes/translates/retries without changing returned
  errors and that `APIDocumentationErrors`-style route consumers still win.
- Confirm bag save status/write ordering with the local fake; do not treat it as
  published kiks evidence.
- Re-run the module commands above on the final tree and then coordinate the
  inertia `ResponseBuffer` implementation and published pins.

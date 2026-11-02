# T02 kiks persistence coverage (development)

Status: **READY FOR INDEPENDENT REVIEW.** Test-and-documentation change only in
`pkg/kiks`. No kiks runtime code changed, no anduin file changed, no module
version, pin, replacement or go.work change, and no commit/tag/publish. The tests
run against the workspace anduin development API; `GOWORK=off` remains red until
that API is published.

Baseline: HEAD `289b1778de20f86210dbec749a1c622ff6018d65` plus the shared
worktree's parallel changes.

## Scope

`pkg/kiks/router_error_test.go` drives the real `Jar.Middleware` through a real
`anduin.Router` and `ErrorHandler`; it does not use `anduin.Serve` as the subject
under test (only for cookie seed/follow-up requests). No inertia dependency or
fake capture is added: these cases assert kiks bag/session/flash persistence, and
the publishable capture-commit contract belongs to the inertia worker. A fake
capture would not exercise a distinct kiks branch.

`pkg/kiks/CHANGELOG.md` documents the contract and the existing `AddFlash`
after-write limitation.

## Pinned contract

| Test | Pinned behavior |
| --- | --- |
| `TestErrorHandlerBagMutationsPersistWithFinalStatus/route_handler_409...` | Route handler error renders after route middleware unwinds but inside kiks. `ErrorHandler` reads the handler's session, sets a new session value, adds a flash, then writes 409. The session store is saved exactly once; one session and one flash cookie are emitted; a follow-up sees the mutated session and persisted flash. |
| `.../global_middleware_409...` | Global middleware error renders at the failing global boundary with live kiks resources. Same reads, mutations, exactly-once save, final 409 and flash persistence. |
| `.../host_middleware_409...` | Host middleware error renders at the failing host boundary with live kiks resources. Same reads, mutations, exactly-once save, final 409 and flash persistence. |
| `.../route_303...` | An `ErrorHandler` flash followed by a 303 redirect is saved with the 303 status and survives a follow-up. |
| `TestForbiddenErrorClearsFlashAndSavesDirtySession` | A 403 `ErrorHandler` may read the bag and dirty the session, but the flash cookie is cleared, not persisted; the dirty session is still saved and observed on a follow-up. |
| `TestCatchErrorsInsideKiksPersistsBagMutations/inside_kiks` | A consuming `CatchErrors` scope installed **after** `Jar.Middleware` reads and mutates the bag, writes its own 409, and never invokes the router `ErrorHandler`; kiks persists the consumer's session and flash with the final status. |
| `.../outside_kiks_has_no_bag_and_saves_before_the_consumer` | The placement control: a scope outside kiks gets `ErrNoBag` from `AddFlash`; the session save count is already 1 when the consumer runs, proving kiks saved before it. A seeded incoming flash is expired by that pre-consumer save, proving the persistence classified status 200 rather than the later 409 render. The delayed render adds no further save. |
| `TestAddFlashAfterResponseWriteIsNotPersisted` | Existing ordering: `persistWriter` saves on the first response write, so `AddFlash` after a 303 is not persisted. Add flashes before writing the response. |
| `TestLoadFailureLeavesNoBagOrCookies` | A kiks session load failure runs the `ErrorHandler` with no bag (`Get`/`AddFlash`/`Set` return `ErrNoBag`), never runs the handler, emits no cookies, and performs no session save. |
| `TestSaveFailureFailsClosedAndLogsJoinedErrorOnce` | An early 403 render with a dirty session hits a save failure. The `ErrorHandler` write error is observable (`errors.Is(saveErr)`), the body is suppressed, kiks joins the original guard error with the save error, and the router logs that changed final error exactly once with both errors visible. There is one render and no second response. |
| `TestAuthOnlyStyleHostRedirectStillPersistsBag` | An AuthOnly-style host middleware destroys the session, stores an encrypted return target, writes a 303 and returns nil. The expired session cookie and the return-target cookie are persisted, and a follow-up reads the stored target. |

All tests are default-on and use only existing kiks seams (`testJar`,
`failSaveStore`, `testKeys`, cookie helpers) plus a counting store and a
failing-load store local to the new test file. No environment variable or skip is
introduced. The current API error-observation contract is preserved: the failing
error still reaches the `ErrorHandler`, and the post-render joined error is
observable in the single log record rather than being swallowed.

## Evidence

Red baseline against the current published dependency set (isolated copy under
`/tmp/opencode/t02-kiks-red/pkg-kiks`, `GOWORK=off`, published anduin v0.6.2,
anduin untouched): build fails because the published API has no `Host.Use` /
`Host.Isolate` and no `CatchErrors`. This is a compile-level red baseline, not
old-runtime behavior proof and not published acceptance.

```text
$ GOWORK=off go test -count=1 ./...
./router_error_test.go:393:9: admin.Isolate undefined ...
./router_error_test.go:554:17: undefined: anduin.CatchErrors
./router_error_test.go:671:17: undefined: anduin.CatchErrors
FAIL	github.com/mbvlabs/andurel/pkg/kiks [build failed]
```

Full log: `plans/rc1-final-readiness/evidence/t02-kiks-red.txt`.

Workspace development run:

```sh
cd pkg/kiks
gofmt -l .                 # no output
go fix ./...               # no output, no source changes
go vet ./...               # exit 0
go test -count=1 ./...     # ok kiks
CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' \
  go test -race -count=1 ./...  # ok kiks
```

Full log: `plans/rc1-final-readiness/evidence/t02-kiks-validation.txt`.

## Known limitations (documented, not changed)

1. **Published API.** `GOWORK=off` cannot build the new tests until the
   coordinated anduin API is published and the kiks pin is updated. This work
   deliberately did not change `pkg/kiks/go.mod`.
2. **AddFlash after write.** The middleware saves on the first response write;
   later bag mutations, including flashes, are not persisted. The test pins the
   existing behavior and the changelog tells callers to flash before writing.
3. **CatchErrors placement.** A scope that needs kiks must be installed inside
   `Jar.Middleware`. An outer scope has no bag on its own context, and the test
   observes the kiks session save before that consumer runs. A seeded incoming
   flash expires under that pre-consumer status-200 save rather than persisting
   with the later 409 render.
4. **Save failure response body.** The first status write still reaches the
   client and the body is suppressed; this is kiks' existing fail-closed writer
   behavior. The test does not ask kiks to invent a second response for a save
   failure.
5. **Inertia capture.** Not covered or imported here; coordinated capture commit
   is a separate worker/module contract.

## Files

- `pkg/kiks/router_error_test.go` — new real-router persistence tests.
- `pkg/kiks/CHANGELOG.md` — Unreleased documentation entries.
- `plans/rc1-final-readiness/t02-kiks.md` — this plan/evidence.
- `plans/rc1-final-readiness/evidence/t02-kiks-red.txt` — isolated old-module
  compile baseline.
- `plans/rc1-final-readiness/evidence/t02-kiks-validation.txt` — workspace
  gofmt/go fix/go vet/module/race results.

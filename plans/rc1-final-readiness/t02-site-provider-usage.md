# T02 site host-error provider/session usage (read-only)

Prepared during the option B coordinate-design pause. No site, inertia or kiks
file was modified.

Snapshot:

```text
site:      /home/mbv/work/andurel-site
site HEAD: 7431d444b04e436912ee95d6ec531184fdfc871f (master, clean when read)
framework: /home/mbv/work/andurel-rc1-work
framework HEAD: 289b1778de20f86210dbec749a1c622ff6018d65
site pins: anduin v0.7.1, inertia v0.14.0, kiks v0.6.1, routing v0.9.1
```

## Actual admin host-error path

The site installs the session jar, the Inertia middleware, and the admin host
guards in this order:

- `app/runtime.go:200-211` — global chain:
  `PreviewSecurityHeaders`, `Recover`, `DenyFraming`, `RealIP`, `Telemetry`,
  `jar.Middleware()`, `renderer.Middleware()`, `CORS`, `CSRF`,
  `RestorePreviewRequestPath`.
- `app/runtime.go:232-238` — `admin.Use(AuthOnly, AdminOnly, AdminOrigin)`;
  `admin.Isolate()`.
- `middleware/auth.go:141-151` — `AdminOnly` returns
  `anduin.Forbidden(ErrAdminRequired)` without writing any cookie, flash or
  response body.
- `middleware/admin_origin.go:35-81` — `AdminOrigin` returns
  `anduin.Forbidden` on unsafe-method provenance failure; no cookie/flash write.
- `middleware/auth.go:26-52` — `AuthOnly` reads `kiks.Get[*Session]` and the
  database user; a database/load error is returned as-is (500 path), not
  mutated into a redirect.

The signed-in non-admin denial (the beta.7 B7-005 host-error case) therefore
reaches the router as `Forbidden(middleware.ErrAdminRequired)` with a normal
kiks bag and session value still on the context.

`handlers.MapError` maps that sentinel to a 403 `Errors/NotFound` page
(`handlers/errors.go:31-43`, `:69-79`). The page is rendered through
`inertia.PageBuilder.Render`, which resolves shared providers and the flash
provider.

## Provider/session paths on the error page

`app/runtime.go:488-503` registers:

- static shared props (`appVersion`, `siteName`, `hosts`, `appUrl`,
  `environment`);
- `inertia.WithSharedProvider(sharedAuth)`;
- `inertia.WithFlashProvider(func(ctx) any { return kiks.Flashes(ctx) })`;
- SSR default/fail-fast policy.

`sharedAuth` (`app/runtime.go:509-530`) does only:

```go
session, err := kiks.Get[*middleware.Session](ctx)
```

It does not call `kiks.Set`, `kiks.Destroy`, `kiks.AddFlash` or `SetCookie`.
The `ErrNoBag` branch returns an anonymous auth prop instead of an error; that
is the beta.7 workaround still present in the site.

The flash provider calls `kiks.Flashes` (`pkg/kiks/flash.go:35-53`), which copies
`bag.flashes` and does not mutate/clear the bag. No error path in
`handlers/errors.go` or `pkg/inertia` calls `AddFlash`.

The Inertia renderer itself writes status/body and `Vary: X-Inertia`
(`pkg/inertia/renderer.go:273-393`); it contains no cookie writes at all
(`grep SetCookie pkg/inertia` is empty). The site does not configure
`WithReflash`/`SetReflashHandler`, so redirect-time flash preservation is not
part of this app's error path (it is still an inertia API contract for other
consumers).

## Direct answer: do site host-error providers mutate cookies/flash?

**No.** The actual host-error sources (`AdminOnly`, `AdminOrigin`, `AuthOnly`
database/load errors) do not write cookies or flashes, and the error-page
providers only read the session and flash cookies. The site can therefore pass
many themed-error tests today with the sharedAuth `ErrNoBag` workaround even
while the router supplies a pre-middleware context.

The one host-middleware bag write is the `AuthOnly` **rejection** path
(`middleware/auth.go:60-92`): it destroys the session, stores a return-to
cookie, and returns `inertia.LocationRedirect` (303, or 409 for a cross-host
Inertia visit). That is a successful redirect, not an error; kiks must continue
to save those mutations after the redirect response.

Consequences for a coordinated flash design:

- Site host errors are 403. `pkg/kiks/middleware.go:113-120` only persists
  flashes for 3xx, 409 and the client-redirect header; a flash added while
  rendering the 403 error page is deliberately cleared, not persisted
  (`pkg/kiks/middleware.go:250-255,313-330`). P2 flash tests must use 409/3xx or
  a synthetic app error, not the site's AdminOnly 403.
- Session cookie persistence on the site's 403 path is not at risk because the
  error providers do not dirty the session bag.
- The site's `sharedAuth` still masks `ErrNoBag` as anonymous. A fixed-context
  acceptance test must assert `auth.isAuthenticated == true` (or use a
  propagating provider), not merely the themed component/status.

## Kiks lifecycle constraints observed

- `Jar.Middleware` (`pkg/kiks/middleware.go:126-172`) loads the bag, replaces
  the writer with `persistWriter`, attaches the bag to the context, calls
  `next`, and only then saves if not already persisted.
- `persistWriter.save` (`:92-98`) calls `jar.persist` with the status seen so
  far. On a returned host/global error that has not written a response, that is
  status 200; this is why the current post-unwind render cannot persist a flash
  or see the effective 409/3xx status.
- If `jar.load` fails (`:141-144`), no bag or `persistWriter` is installed at
  all. An ErrorHandler cannot see a bag, and kiks cannot save anything for that
  request.
- `persistErr` after `next` is returned alone when `err == nil`, or joined with
  the handler error when non-nil (`:163-168`). Any coordinated render-before-save
  design must define what happens if persist fails after an error page has
  committed; current kiks tests (`TestPersistSaveErrorFailsRequest`,
  `TestPersistMarshalErrorFailsRequest`) only exercise `anduin.Serve`, not the
  router ErrorHandler.
- `Jar.Middleware` never restores the writer. After it unwinds,
  `anduin.ResponseWriter(ctx)` is still its `persistWriter`; an error render must
  use that writer for cookies to persist.

## Inertia lifecycle constraints observed

`Renderer.Middleware` (`pkg/inertia/middleware.go:16-71`) only captures the
writer for Inertia visits (`X-Inertia`); non-Inertia browser documents go
straight to the writer (`:31-33`). For Inertia visits it:

1. saves `original := anduin.ResponseWriter(ctx)`;
2. installs an unexported `captureWriter`;
3. calls `next`;
4. restores the original writer;
5. returns immediately if `err != nil` without committing the capture (`:54-58`);
6. otherwise normalizes, optionally reflashes, and commits (`:60-68`).

Therefore:

- A render performed inside the Inertia boundary into `captureWriter` is lost
  when the error propagates, unless inertia commits it.
- Returning nil from the boundary makes inertia commit, but changes error
  observation for every upstream middleware and would let route middleware
  commit/return success; this is the earlier design conflict.
- Browser (non-Inertia) admin errors do not use `captureWriter`; a
  render-before-kiks-save design works for those without an inertia commit, but
  the site's admin tests use Inertia visits (`c.inertia()`), so capture is
  exercised.
- `captureWriter` and `persistWriter` are unexported. Coordinated tests must
  drive them through `Router.Handler`, not type-assert internals.

## Existing test seams and gaps

Site seams:

- `app/admin_dashboard_test.go`: `newDashboardClient`, `authRuntime`,
  `signIn`, cookie replay; `TestAppAdminDashboardAuthorizationMatrix` drives a
  demoted admin to a themed 403. It does not assert the error page's
  `auth` prop or any error-page cookie.
- `app/admin_framing_test.go`: real app, demoted admin response, themed
  component and framing headers; no auth prop/cookie assertion.
- `handlers/host_matrix_test.go`: real kiks jar + host guards + `MapError` on an
  `anduintest` server; good host-error seam, but the test renderer has no
  `sharedAuth` provider.
- `handlers/main_test.go`: `testRenderer`/`testFlashRenderer`, flash provider
  wired to `kiks.Flashes`; no shared provider.
- `handlers/api_documentation.go:35-43` and
  `handlers/api_documentation_error.go:27-37`: `APIDocumentationErrors`
  is a route middleware that consumes a returned error into a JSON response and
  returns nil. A boundary design that renders at the deepest next before that
  middleware sees the error would bypass the API JSON contract and double-render.
  This is a concrete site contract any "render earlier" design must preserve.
- No site test asserts signed-in chrome on a host-error page, and no site error
  handler adds a flash.

Inertia seams:

- `pkg/inertia/renderer_test.go:21-25` `testContext`, `:30` `newTestRenderer`,
  `:57` `testRoot`; `error_handler_test.go` `errorHandlerRequest`; `fakeSSRRenderer`.
- There are **no** tests for `Renderer.Middleware` error/capture behavior, no
  `WithSharedProvider` tests, no `WithFlashProvider` tests, and no `WithReflash`
  tests. `Middleware` is only exercised for nil-error logging/config paths.
- A coordinated red test in inertia can import anduin (already a dependency)
  and drive a `Router` with a custom ErrorHandler; a combined kiks+inertia test
  needs a package that depends on both (site or a dedicated integration module),
  because inertia does not depend on kiks and kiks should not gain a test-only
  inertia dependency.

Kiks seams:

- `pkg/kiks/kiks_test.go` has `testJar`, `cookieByName`, `cookieNames`;
  `failSaveStore`/`failMarshalApp` model persist failures; all current tests use
  `anduin.Serve`, not `anduin.Router`/`ErrorHandler`.
- No kiks test covers a returned handler error, an ErrorHandler that calls
  `AddFlash`, 409 flash persistence, or save ordering relative to an error
  render. Those are the tests a coordinated fix needs.

## Constraints not already captured by the current T02 note

1. The site's host-error path is read-only and 403; P2 flash persistence must be
   tested with a synthetic 409/3xx error, not the AdminOnly path.
2. `AuthOnly` mutates the bag on its successful redirect path; redirect save
   ordering must survive any error lifecycle change.
3. `sharedAuth` masks `ErrNoBag`; acceptance must assert authenticated chrome or
   use a provider that propagates the error.
4. `APIDocumentationErrors` consumes route errors and must run before any
   router ErrorHandler render; naive deepest-boundary rendering breaks the site's
   JSON API contract.
5. Inertia capture applies only to Inertia visits; browser-document and Inertia
   error paths need separate coverage.
6. Kiks `jar.load` failure leaves no bag/writer; ErrorHandler context is
   necessarily restricted there. Kiks save failures after an error render need
   an explicit policy.
7. The default-skipped T02 design-hold tests are **not** final acceptance. Any
   fix must enable them (or replace them with dedicated integration tests) and
   run the published-module T09 repro; publication remains unauthorized.

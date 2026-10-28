# Fix plan: friction from the andurel-site port (v2.0.0-beta.2)

Source: `../andurel-site/plans/next-site/friction/` — mainly `N32-assets.md`,
`N30.md`, `N30A-N31A.md`, `N50-cors-fix.md`, `N50-frame-hardening.md`.
Every item below was checked against current source (oracle triage, see
"Verdicts"). Target release: v2.0.0-beta.3.

Rules for every lane: `gofmt`, `go vet`, `go fix`; no python/perl; use the
edit/write tools. Changes under `pkg/` get a CHANGELOG entry (Unreleased
section) in the touched module. Scaffold changes update every adapter variant
where the template exists per adapter, and the goldens (`just test-golden`).

## Decisions (settled)

- **D1 — 405 goes through the ErrorHandler.** ServeMux-generated 405s become
  `anduin.ErrMethodNotAllowed`; the `Allow` header is preserved. The existing
  `TestRouterMethodNotAllowedIsPreserved` is rewritten (its real intent — a
  405 must not turn into a 404 — stays asserted).
- **D2 — Vite assets get non-credentialed CORS.** The asset route answers with
  `Access-Control-Allow-Origin: *` and no `Access-Control-Allow-Credentials`.
  `pkg/inertia` keeps emitting a bare `crossorigin` (anonymous mode); a test
  locks that in.
- **D3 — No `Router.UseHost`.** Host-scoped policy is ordinary global
  middleware that branches on `anduin.RequestHost(ctx)`.
- **D4 — Ryuk is upstream behaviour.** testcontainers-go v0.43 shares one
  session per parent `go test` process. We document; `storagetest` does not
  try to change the session id.
- **D5 — A 405 renders the not-found page.** The scaffold maps
  `anduin.ErrMethodNotAllowed` to status 405 rendering `Errors/NotFound`.
  No new error page.

## Verdicts (summary)

| # | Finding | Verdict | Lane |
|---|---|---|---|
| 1 | `StaticFS`/`ServeContent` replace `Vary` | framework bug | A1 |
| 2 | 304 skips `Vary: Accept-Encoding` | framework bug (low) | A1 |
| 3 | Scaffold installs credentialed CORS for every host origin | scaffold security default | B1 |
| 4 | No authoritative host resolver; host match is case-sensitive | API gap + minor bug | A2 |
| 5 | Mux 405 bypasses ErrorHandler | gap (D1) | A2 + B1 |
| 6 | No page URL override for POST-rendered pages | API gap | A1 |
| 7 | Root template cannot see the host | docs, once A2 lands | B2 |
| 8 | `sync payloads` skips `FromStruct(param)` | CLI bug | A3 |
| 9 | `verify` hides per-test skips | CLI gap | A3 |
| 10 | Ryuk reaps sibling package containers | upstream; docs | B2 |
| 11 | CSRF sets `Set-Cookie` on immutable assets | scaffold | B1 |
| 12 | Inertia `<Link>` used for cross-host URLs | docs | B2 |
| 13 | `anduintest` has no host support | API gap | A2 |
| 14 | `RouteWithParams` types numeric params as strings | deferred | — |

Not ours: Tiptap `setEditable` source loss, Chrome rAF suspension in
background tabs, CDP `blockedReason=other` for CSP frame blocks.

---

## Lanes and ordering

```
Phase A (parallel, disjoint files)       Phase B (after A merges)
  A1  pkg/anduin Vary + pkg/inertia  ─┐
  A2  pkg/routing + pkg/anduin router ├─▶ B1 scaffold security + upgrade notes
  A3  cli payloads + verify          ─┘   B2 docs (readme, agents, skills)
                                          C  release pkg modules, pin, beta.3
```

A1 and A2 both touch `pkg/anduin/CHANGELOG.md`; whichever merges second
resolves that one-file conflict. B1 and B2 both depend on the A APIs being
merged (they reference `RequestHost`, `ErrMethodNotAllowed`, `AddVary`).
B1 and B2 do not share files: B1 owns all `.tmpl` except `readme.tmpl` and
`agents.tmpl`; B2 owns those two plus `skills/`.

---

## A1 — Vary merging and `PageBuilder.URL` (`pkg/anduin`, `pkg/inertia`)

Files: `pkg/anduin/static.go`, `pkg/anduin/compress.go`, new
`pkg/anduin/vary.go` (or put it in `response.go`), `pkg/anduin/static_test.go`,
`pkg/inertia/renderer.go`, `pkg/inertia/*_test.go`, both CHANGELOGs.

### A1.1 Exported Vary merge helper
- Add `func AddVary(header http.Header, tokens ...string)`. It appends each
  token unless it is already present (case-insensitive, comma-split, trimmed,
  first-seen spelling kept). `*` is not special-cased.
- Replace `compress.go` `addVary` and `inertia/renderer.go` `appendVary` with
  it. Delete both private copies.

### A1.2 Static serving merges instead of replacing
- `serveContent` (`static.go:128-143`): replace
  `SetHeader(ctx, "Vary", "Accept-Encoding")` with `AddVary` on the response
  header, and move it **above** the `If-None-Match` 304 return so 304 and 200
  carry the same `Vary`.
- Grep `pkg/` for any other `Set`/`SetHeader` on `Vary`; there should be none
  left.

### A1.3 Tests (red first)
- In `static_test.go`: router with `CORS(DefaultCORS("http://admin.example"))`
  and `StaticFS(..., Immutable(true))`. Table: GET / HEAD / conditional 304 ×
  with and without `Origin`. Assert `Vary` contains both `Origin` and
  `Accept-Encoding` in every case, and ACAO is set only when `Origin` is
  allowed. Run it with and without `Compress` wrapping the router (the 304
  bug only shows with compression off).
- `ServeContent` 304 test: `Vary: Accept-Encoding` present.
- `AddVary` unit tests: dedupe across multiple header lines, case, whitespace,
  empty header.

### A1.4 `PageBuilder.URL(string)` (`pkg/inertia`)
- Add a builder method that overrides the page object's `url` (otherwise
  `requestURL(ctx)`, `renderer.go:451`). Use case: a POST handler renders a
  success page and wants the browser URL to be a GET-able route.
- Applies to the JSON (X-Inertia) and the full-document/SSR responses alike.
- Test: POST request, `.URL("/users/confirmation/new")`, assert `page.url`
  in both the JSON response and the document page script.

### A1.5 Lock in anonymous `crossorigin` (D2)
- Test in `pkg/inertia` asserting production tags emit `crossorigin` with no
  value (or `crossorigin="anonymous"`), never `use-credentials`. Add a code
  comment at `config.go:133,138` pointing to the scaffold's non-credentialed
  asset CORS.

Acceptance: `go test ./pkg/anduin/... ./pkg/inertia/...` green; CHANGELOGs
list `AddVary`, the static Vary fix, the 304 fix and `PageBuilder.URL`.

---

## A2 — Host resolution, 405, anduintest hosts (`pkg/routing`, `pkg/anduin`)

Files: `pkg/routing/hosts.go` (+ test), `pkg/anduin/router.go`,
`pkg/anduin/context.go`, `pkg/anduin/errors.go`, `pkg/anduin/router_test.go`,
`pkg/anduin/anduintest/*`, CHANGELOGs for routing and anduin.

### A2.1 `routing.Hosts.Resolve`
- `func (h *Hosts) Resolve(authority string) (HostName, bool)`.
- Rules (move them here from `anduin/router.go:255-312`; the router must call
  this, not keep its own copy):
  1. Trim, lowercase the authority (hostnames are case-insensitive; browsers
     lowercase, proxies and curl may not).
  2. Exact match against configured hostnames and aliases (lowercased at
     index time).
  3. Otherwise `net.SplitHostPort` and match the bare host. This keeps the
     current "bare hostname also claims `:80`/`:443`" behaviour. Decide and
     test whether a bare configured hostname should also match an arbitrary
     port (`admin.example:8080`): **today it does** (the split fallback), so
     keep that and document it.
  4. IPv6 literals (`[::1]:8080`) resolve via the split path.
  5. Unknown → `("", false)`. Nil `*Hosts` → `("", false)`.
- Index is built once in `NewHosts` (it's immutable after construction), so
  `Resolve` needs no lock. The duplicate-claim check in `NewHosts` must
  compare lowercased names too (`Admin.example` and `admin.example` on two
  HostNames is a boot error).
- Changing the router's host matching is a behaviour change. Note it in the
  CHANGELOG.
- Table test: exact host, alias, `host:port` configured explicitly, bare host
  with `:80`, `:443`, `:8080`, uppercase, IPv6, unknown, empty.

### A2.2 Router uses `Resolve`; host on the context
- Delete `hostIndex`/`indexHostsLocked`; `muxForRequest` calls
  `rt.hostsRegistry.Resolve(r.Host)`, falling back to `HostPrimary` and then
  to `rt.primary` when that host has no mux (unchanged behaviour).
- In `serve()`, resolve **before** `applyMiddleware(base, rt.mw)` and store the
  result on `requestState` so global middleware can read it.
- `func RequestHost(ctx context.Context) routing.HostName` returns the
  resolved name, or `routing.HostPrimary` for unknown hosts, no registry, or a
  context without router state. Doc comment: this is the same answer the
  dispatcher uses; it reports the configured host even when that host has no
  registered routes.
- `muxForRequest` should reuse the stored value instead of resolving twice.
- Tests: global middleware sees `RequestHost == admin` for an admin request
  including an unmatched path (404) and a 405; uppercase `Host` dispatches to
  the admin mux (regression for the case bug).

### A2.3 `ErrMethodNotAllowed` (D1)
- `errors.go`: `var ErrMethodNotAllowed = errors.New("anduin: method not allowed")`;
  `StatusCode` maps it to 405.
- `notFoundWriter` (rename to `muxStatusWriter` or similar) also intercepts
  `405` when `request.Pattern == ""`, records it, swallows the mux body, and
  `serve()` calls `rt.handleError(ctx, ErrMethodNotAllowed)`. The `Allow`
  header is already set by ServeMux before `WriteHeader`; verify it survives
  into the final response, and that the ErrorHandler's renderer does not
  clear it.
- Without an ErrorHandler, `defaultErrorHandler` must still produce a 405 with
  `Allow` (no behaviour change for apps that don't map it).
- Rewrite `TestRouterMethodNotAllowedIsPreserved`: ErrorHandler is called with
  `ErrMethodNotAllowed`; status 405; `Allow: GET, HEAD`; and a separate case
  showing it's never reported as `ErrNotFound`.

### A2.4 `anduintest` host support
- `anduintest.WithHosts(*routing.Hosts) Option` — applies `anduin.WithHosts`
  to the router it builds (today tests call `anduin.WithHosts(hosts)(rt)`
  inside the register callback).
- Per-request host: `Server.Do` already takes `*http.Request`; add a
  `Host(authority string)` request option, or a `Header{Key: "Host"}` special
  case that sets `req.Host` (Go's client ignores a `Host` header). Pick one,
  document it in the package doc.
- Test: admin-host route reachable only with the admin host option.

Acceptance: `go test ./pkg/routing/... ./pkg/anduin/...` green; CHANGELOGs list
`Hosts.Resolve`, `RequestHost`, case-insensitive host matching (behaviour
change), `ErrMethodNotAllowed` (behaviour change), anduintest options.

---

## A3 — CLI: payload scanner and verify skips (`cli/`)

Files: `cli/payloads_generation.go` (+ test), the `sync payloads` command
definition, `cli/verify.go` (+ test), CLI help/metadata strings, and
`skills/andurel/references/cli-catalog.md` only for these two commands.

### A3.1 Resolve function and closure parameters
- `collectFuncLocals` (`payloads_generation.go:923`) only walks `fn.Body`.
  Seed it with `fn.Type.Params` (name → param type expr, same shape as the
  typed `ValueSpec` case), and while walking, add params of every
  `*ast.FuncLit`. Scoping is flat today (one map per FuncDecl); keep it flat,
  first declaration wins is fine, but don't let an inner closure param
  shadow an outer local of the same name with a different type — if names
  collide with different types, treat as unresolved.
- Also handle `*T` param types (`FromStruct(*props)` / pointer params) the
  same way existing locals are handled.
- Fixtures: top-level helper `func render(props XProps)`, closure
  `render := func(props XProps) error {...}`, pointer param. Each emits the
  type with zero skips.

### A3.2 Strict mode
- `andurel sync payloads --strict`: any `skipped` entry with reason
  `unresolved FromStruct argument` fails with a non-zero exit code and the
  list of `file:line` locations. JSON output keeps the same shape plus an
  error.
- Non-strict human output prints each unresolved root's `file:line` (not just
  a count) with a hint: "pass a named composite literal or a typed variable".
- `andurel verify` runs sync drift checks; decide whether its payload drift
  check uses strict. Recommendation: verify **reports** unresolved roots as a
  warning in its JSON (`warnings`) but does not fail on them in beta.3.

### A3.3 Per-test skips in `verify --with-tests`
- `verify.go:396-470` already parses `go test -json`. Collect
  `Action == "skip"` events that have a `Test` name into
  `SkippedTests []verifySkippedTest` (`package`, `test`, `reason` from the
  last `output` line containing `SKIP` or the `t.Skip` message).
- JSON: `skipped_tests` on the go test step. Human output: `N tests skipped`
  plus the list. The step stays `ok` (no failure), mirroring `skipped_packages`.
- Update `verify --help` and the long description (line 138) and the
  `andurel commands --json` metadata.
- Do **not** build the frontend inside `verify`.
- Test: fixture package with one passing and one `t.Skip("reason")` test.

Acceptance: `go test ./cli/...` green; `cli-catalog.md` documents `--strict`
and `skipped_tests`.

---

## B1 — Scaffold security composition (`layout/`)

Depends on A1 + A2 being merged (use `RequestHost`, `ErrMethodNotAllowed`).
Files: `layout/templates/runtime_runtime.tmpl`, `handlers_assets.tmpl`,
`handlers_errors.tmpl`, `router_middleware_*.tmpl` (new helpers),
`runtime_runtime_test.tmpl`, `config_http.tmpl` if needed,
`go_mod.tmpl` pins (in phase C), `layout/upgrade/upgrade_notes.go` (+ test),
goldens.

### B1.1 Scope CORS (D2)
- Global CORS (`runtime_runtime.tmpl:116-133`) trusts **only**
  `cfg.HTTP.CORSAllowedOrigins`, not `cfg.App.Origins()`. When that list is
  empty, still install it (harmless, keeps the wiring visible) or skip it —
  pick one and comment why.
- Global CORS gets a `Skipper` for the Vite asset prefix
  (`routes.ViteBuild` path) so it never adds credentialed headers to assets.
- The asset route registers its own middleware:
  `anduin.CORS(anduin.CORSConfig{AllowOrigins: []string{"*"}, AllowMethods: GET/HEAD})`
  with `AllowCredentials: false`. With A1, `Vary` stays merged anyway.
- Keep `cfg.App.Origins()` in **CSRF** trusted origins (same-site form posts
  between hosts are a deliberate feature; CSRF is not a read policy).
- Runtime tests (`runtime_runtime_test.tmpl`), with a second host configured:
  - Admin document requested with `Origin: <primary>` → no ACAO, no ACAC.
  - Primary document requested with `Origin: <admin>` → no ACAO.
  - Asset with `Origin: <admin>` → `ACAO: *`, no ACAC, `Vary` has `Origin`
    and `Accept-Encoding`.
  - A `CORS_ALLOWED_ORIGINS` origin still gets credentialed CORS on non-asset
    routes.

### B1.2 No CSRF cookie on assets
- CSRF `Skipper` additionally skips **safe-method** requests under the asset
  prefix (compose with the existing `SkipCSRFForBearer`). Unsafe requests are
  never skipped by this rule.
- Test: asset GET has no `Set-Cookie`; a document GET still gets the cookie.

### B1.3 405 renders the themed page (D1, D5)
- `handlers_errors.tmpl`: add
  `inertia.MapError(anduin.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "Errors/NotFound")`.
- Runtime test: `GET` on a POST-only route → 405, `Allow` header present,
  themed `Errors/NotFound` (Inertia and document).

### B1.4 Multi-host security header example (optional, recommended)
- Ship `middleware.DenyFraming(hosts ...routing.HostName)` in the scaffold
  middleware template, using `anduin.RequestHost(ctx)`. It sets
  `X-Frame-Options: DENY` and `Content-Security-Policy: frame-ancestors 'none'`
  before `next` so errors, 404 and 405 carry them.
- Scaffold does **not** wire it by default (single-host apps), but the
  readme (B2) shows the one-line `router.Use(...)` for an admin host. If
  shipping unwired code is against scaffold policy, move it to B2 as a
  documented snippet instead.

### B1.5 Upgrade notes
- New `betaThreeFixesVersion = "v2.0.0-beta.3"` manual actions:
  1. Narrow global CORS to `CORS_ALLOWED_ORIGINS`, skip the asset prefix, add
     uncredentialed asset CORS (show the diff). Flag it as security-relevant
     for apps with more than one host.
  2. Skip CSRF cookie issuance for safe asset requests.
  3. Map `anduin.ErrMethodNotAllowed` in `MapError`.
  4. Apps that mirrored host matching (like andurel-site's
     `middleware/host_authority.go`) can switch to `anduin.RequestHost`.
- Test in `upgrade_notes_test.go` that beta.2 → beta.3 yields these actions.

Acceptance: fresh `andurel new` (all adapters) passes `andurel verify
--with-tests`; `just test-golden` green with updated goldens.

---

## B2 — Docs and guidance (`readme.tmpl`, `agents.tmpl`, `skills/`)

Depends on A1–A3 APIs and B1 decisions (can run in parallel with B1 once
the B1 shape above is accepted). Files: `layout/templates/readme.tmpl`,
`layout/templates/agents.tmpl`, `skills/andurel/references/http-and-routing.md`,
`skills/andurel/SKILL.md` if it indexes these topics, `docs/` only if a page
already covers the topic.

### B2.1 Readme "Session, CORS, And CSRF Protection" + "Multiple Domains"
- Rewrite the CORS paragraph (`readme.tmpl:744-748`) to the B1 model:
  explicit origins only, credentialed; assets public and uncredentialed;
  why host origins are **not** in CORS (a sibling host must not read
  authenticated responses).
- Add "Host-aware middleware": `anduin.RequestHost(ctx)`, resolution rules
  (case-insensitive, aliases, default ports, unknown → primary), that global
  middleware sees it before dispatch, and the `DenyFraming` example for an
  admin host.
- Add "Host-aware root template": `root.templ` renders with the request
  context, so `anduin.RequestHost(ctx)` works there; example: withhold
  analytics on the admin host. Replaces the component-name heuristic.
- Add "Cross-host links": generated helpers return absolute URLs for
  host-bound routes; use a plain `<a href>` when the target host differs,
  Inertia `<Link>` only for same-host navigation. Short snippet per adapter
  is optional; one React example is enough.
- Mention `PageBuilder.URL` under the pages/forms section: render-after-POST
  pages should set a GET-able URL.

### B2.2 Readme "Testing"
- Test DB section: testcontainers shares one Ryuk session per `go test`
  invocation; under parallel packages a finished package can reap a sibling's
  container. Use `go test -p 1 ./...` or `TESTCONTAINERS_RYUK_DISABLED=true`
  (`Cluster.Close` still terminates containers; a crashed run may leave one
  behind).
- Handler tests: `anduintest.WithHosts` and the per-request host option.
- Verify: `skipped_tests` exists; tests that need `assets/dist` should
  `t.Skip` with a message and the final evidence run builds the frontend
  first (`pnpm build`).

### B2.3 `agents.tmpl` and skill references
- Agent rules (short): cross-host navigation uses `<a>`; host-scoped policy
  uses `anduin.RequestHost`, never re-parses `Request.Host`; pass named
  composite literals or typed params to `FromStruct` and run
  `andurel sync payloads --strict`.
- `http-and-routing.md`: update the CORS composition example (`:264-280`),
  document `AddVary`, `RequestHost`, `Hosts.Resolve`, `ErrMethodNotAllowed`,
  and that `StaticFS`/`ServeContent` merge `Vary`.

Acceptance: docs match the shipped templates (grep the readme for
`MergeOrigins(cfg.App.Origins(), cfg.HTTP.CORSAllowedOrigins)` — it must not
appear in CORS context anymore); goldens updated for readme/agents.

---

## C — Release (maintainer, sequential)

Follow `docs/releasing.md`:
1. Release `pkg/routing` (minor: `Resolve`), then `pkg/anduin` (minor:
   `AddVary`, `RequestHost`, `ErrMethodNotAllowed`, behaviour changes), then
   `pkg/inertia` (minor: `PageBuilder.URL`, depends on anduin).
2. Bump inter-module requires (`pkg/inertia/go.mod` → new anduin, etc.) and
   pin the new versions in `layout/templates/go_mod.tmpl`.
3. Release notes in `docs/release-notes/` for beta.3: call out the CORS scope
   change and the 405 behaviour change as the two things existing apps must
   act on.
4. Re-run the andurel-site against beta.3 and delete its workarounds per
   their removal conditions: `handlers/assets_vary.go`,
   `middleware/host_authority.go` (→ `RequestHost`), the CORS skipper, the
   analytics component-name rule, the `history.replaceState` in
   `Pages/Auth/Confirmed.tsx` (→ `PageBuilder.URL`), the helper-literal
   payload workaround.

## Deferred

- `RouteWithParams` numeric param typing (N30-F04).
- A generated `RouteLink`/`sameOrigin()` helper for cross-host links.
- Lazy CSRF cookie issuance inside `pkg/anduin` (B1.2 covers the asset case).
- Runtime test option for dev SSR (`INERTIA_SSR_FAIL_FAST`) (N30A-F5).

# Fix plan: friction from the andurel-site port (v2.0.0-beta.1)

Source: `../andurel-site/plans/friction/*.md` (19 files, ~150 entries, many
"seen again"). This plan de-duplicates them into framework work items, grouped
by priority. Orchestration-only issues are listed under "Out of scope" at the
end.

Rules for every phase: `gofmt`, `go vet`, `go fix`; changes under `pkg/` get a
CHANGELOG entry and a v0.x minor/patch bump; scaffold changes must update the
golden tests (`golden/`) and every adapter variant (react, svelte, vue) where
the template exists per adapter.

---

## Phase 1 — Blockers (scaffold ships broken behaviour)

### 1.1 Default email sender is an invalid address
- Problem: `config_email.tmpl` defaults `DEFAULT_SENDER_SIGNATURE` to
  `"noreply@"+app.Domain`, and `Domain` includes the port
  (`noreply@localhost:8080`). Every email job is cancelled by
  `pkg/email` validation; nothing is logged. A fresh scaffold cannot send
  auth emails.
- Fix:
  - `layout/templates/config_email.tmpl`: derive the default from the host
    without the port (`net.SplitHostPort` fallback), and validate the final
    value with `mail.ParseAddress` in `NewMail` so a bad value fails at boot.
  - `layout/testdata/configruntime/main.go`: update the assertion
    (currently asserts the broken `noreply@localhost:8080`).
  - `layout/templates/env.tmpl`: fix the comment.
- Related (1.2): make the failure visible.

### 1.2 Failed email jobs are silent
- Problem: permanent errors cancel the River job with no log line in
  `cmd/app` or `cmd/queue`.
- Fix: email workers (`psql_queue_workers_send_*.tmpl`) log at error level
  with job id/kind before returning a cancel error. Consider a small shared
  helper in the queue template rather than repeating the log in each worker.

### 1.3 Unmatched paths: home is a catch-all, and 404 is plain text
- Problem: `GET /` matches everything; registering `/{$}` breaks
  `routes.ts` (`path: () => '/{$}'`) and hides the route from
  `inspect handlers`. Unmatched paths that *don't* hit a route get
  `http.ServeMux`'s plain-text 404, never the app's `Errors/NotFound`.
- Fix:
  - `pkg/routing`: add an exact-match option (e.g. `routing.Exact()`) on
    route values. `Path()` keeps returning the URL (`/`); a new
    `Pattern()` returns the mux pattern (`/{$}`). `pkg/anduin`'s
    registration uses `Pattern()` when given a route value.
  - `pkg/anduin`: add `WithNotFoundHandler` / route the mux's
    not-found through the router `ErrorHandler` (return
    `anduin.ErrNotFound`-style sentinel) so apps get the themed 404
    without a hand-written `/{path...}` catch-all.
  - Scaffold `router_routes_pages.tmpl` + `handlers_pages.tmpl`: home route
    uses `Exact()`; error handler maps the router's not-found to
    `Errors/NotFound`.
  - `cli/routes_js_generation.go`: emit `Path()` (never `{$}`).
  - `cli/route_manifest.go` / `handler_inspect.go`: surface `exact: true`
    and the mux pattern.
- Docs: http-and-routing reference gets a short "pattern precedence"
  section and "`Get` also serves HEAD".

### 1.4 `andurel build` builds only `cmd/app`
- Problem: production needs `cmd/app`, `cmd/queue`, `cmd/migrate`, and
  `cmd/ssr`; build emits one binary, says nothing about the rest, and the
  help text is wrong ("Builds Tailwind CSS", "the production binary").
- Fix (`cli/build.go`):
  - Build every `cmd/*` except `seeds` by default into `bin/` (or `dist/`),
    with `--only app,queue` to narrow.
  - Print an artifact summary at the end (path, size, version), and
    support `--json` with an artifact manifest.
  - Correct the help text: what is built, where it lands, that client
    assets are embedded, and that `ssr.js` is embedded for `cmd/ssr`.
  - Pass `--version` into telemetry as well (see 3.6).
  - Output dir goes into the scaffold `.gitignore` (`gitignore.tmpl`).

### 1.5 `kiks.Destroy` drops flashes, so sign-out cannot flash
- Problem: `Destroy` nils the flash bag and `persist` expires the flash
  cookie when the session key is deleted.
- Fix (`pkg/kiks`): decouple flash persistence from session deletion —
  flashes added after `Destroy` are written to the flash cookie;
  flash load/persist also must not depend on `jar.sessionDef != nil`
  (1.5b). `AddFlash` returns an error (or document + log) when no bag is
  on the context. Add tests for destroy-then-flash and flash-only jars.
  Also fix `Flashes` returning a typed nil slice, so it serialises as an
  omitted field rather than `"flash": null`.

---

## Phase 2 — SSR and production story

### 2.1 Error pages cannot opt into SSR
- Decision: CSR stays the default; SSR is opt-in per page with `.SSR()`.
  No renderer-wide default.
- Problem: `inertia.ErrorHandler` builds its pages internally and calls
  `.Render()` without `.SSR()`, with no way to opt in. The site had to
  copy `MapError` (including private `wantsJSON`/`fieldErrors`) into
  `handlers/errors.go` to get SSR'd error pages.
- Fix (`pkg/inertia`): an `SSR bool` field on `ErrorMapping` (per mapped
  sentinel) plus an `ErrorHandler` option for the validation/fallback
  pages, so error pages opt in the same way handlers do.
- Docs: http-and-routing reference states CSR is the default, `.SSR()`
  opts in per render, and every re-render branch (validation errors,
  rate-limit responses) needs its own `.SSR()` to stay server-rendered.

### 2.2 Embedded SSR bundle cannot run (bare imports)
- Note: the site's own fix (`ssr: { noExternal: true }` unconditionally,
  andurel-site `a17ae00`) broke dev SSR under `andurel run`; the friction
  entry overstates it as a framework finding. Only the non-self-contained
  production bundle is a framework bug. QA never loaded a page in dev mode,
  so it went unnoticed.
- Fix: scaffold `vite_config` templates (all adapters) set
  `ssr.noExternal: true` **only for the production SSR build**
  (`command === 'build'`, e.g. `ssr: command === 'build' ? { noExternal:
  true } : undefined`), never for the Vite dev server, so `ssr.js` is
  self-contained without touching dev SSR. Document the size tradeoff.
- Test: golden or e2e check that does both (a) runs the built
  `cmd/ssr` from an empty directory and (b) renders an SSR page under
  `andurel run` / Vite dev. Add "load a page in dev mode" to
  `docs/release-verification.md`.
- Fix `resolveSSRBundle` so a missing absolute
  `INERTIA_SSR_BUNDLE` still falls back to the embedded copy, and say so
  in the error when the fallback also fails.

### 2.3 Dev SSR transport is undocumented; fail-fast default is surprising
- Fix:
  - Document in README/skill: in development SSR goes through Vite
    (`pnpm dev` must be running); in production through `cmd/ssr`.
  - Scaffold `.env` for development: `INERTIA_SSR_FAIL_FAST=false`;
    production default stays `true`, documented next to the flag.
  - With fail-fast, the SSR error renders through the app's error handler
    (themed 500), not the router's plain text.
  - `cmd/app` logs a warning at boot when SSR is enabled in production and
    the sidecar `/health` isn't reachable.

### 2.4 `andurel doctor` ignores `ENVIRONMENT` for the SSR check
- Fix (`cli/doctor.go`): read `ENVIRONMENT` from `.env`/env; in
  production, require `assets/dist/ssr/ssr.js` (or the embedded copy).

### 2.5 Root template duplicates `<title>` under SSR
- Fix (`inertia_framework_root.tmpl`): render the fallback title only when
  `data.SSR == nil`. Document `RootData` fields in the skill.

### 2.6 Production-like local run needs fake SMTP
- Fix: startup validation in `runtime` only rejects `mailpit` in production
  when `EMAIL_PROVIDER` was explicitly set; add a documented
  `EMAIL_PROVIDER=log` (writes to stdout) for local production runs and
  smoke tests. README gets a "run production locally" recipe.

### 2.7 `cmd/queue` validates the whole web config
- Fix: split `config.New()` into what the queue needs (DB, queue, mail,
  telemetry) vs web-only; `cmd/queue` uses the narrower constructor.

### 2.8 Deploy reference
- Add a `deploy` pattern (`andurel patterns show deploy`) and a skill
  reference: the four processes, start order (`migrate` → `ssr` → `app` +
  `queue`), env each needs, health endpoints, artifacts from 1.4.

---

## Phase 3 — Discovery, docs and help-text drift

### 3.1 Frontend has no verify stage
- Fix: scaffold `package.json` templates get `typecheck` (`tsc --noEmit`,
  `svelte-check` for svelte). `andurel verify` gains a `frontend` stage
  that runs `typecheck` when `package.json` has it (skippable via flag).
  Optional later: lint/format.

### 3.2 `verify` is whole-tree only
- Fix: `andurel verify [paths...]` scopes `gofmt`/`vet`/`fix`/layers to the
  given packages; generated-drift stays global but labels which source
  file each drift belongs to.

### 3.3 Email `_templ.go` has two generators; verify can't see drift
- Fix: `andurel verify` `generated` stage includes email templates and
  compares against `andurel sync email` output (not plain templ).
  `andurel sync email --help` states it writes `email/*_templ.go`;
  `--json` returns a file list. The scaffold's committed email
  `_templ.go` files are regenerated by the current compiler (goldens).
  The `email` pattern says "regenerate with `andurel sync email`, never
  `templ generate`".

### 3.4 `sync payloads` and page props
- Fix: follow named nested structs across packages in the project module
  (not just `handlers/`), and document the rules in `--help`: what is
  scanned (`DecodeJSON`/`DecodeForm`/`FromStruct`), the `Payload` suffix
  requirement, what is rejected. Fix SKILL.md claims accordingly. Remove
  "Bind structs" wording.

### 3.5 `routing.InertiaRoute()` is the only way to get a JS helper
- Fix: add `routing.JSRoute()` (or rename to `TypeScriptRoute()` with the
  old name kept as an alias). Generated helpers resolve origin lazily so
  module-scope constants still get the host; until then, document "call
  helpers at render time".

### 3.6 Config surface is undiscoverable
- Fix:
  - `env.tmpl` (`.env.example`) lists every key the config reads, with
    defaults, grouped per file (INERTIA_*, DB_*, HTTP_*, QUEUE_*,
    TELEMETRY_*).
  - `andurel inspect config --json`: static scan of `e.String/Bool/...`
    calls in `config/` returning key, default, file.
  - `TELEMETRY_SERVICE_VERSION` defaults to the build version.
  - Add `SITE_NAME` (display name) to `config_app.tmpl`, shared as a prop.

### 3.7 Skill and pattern drift
- `patterns show composition`: match scaffold (`Register(rt)`,
  `WithHosts`, no `app/config`).
- `patterns show route`: show the real flat `routes.ts` output and the
  name → key rule.
- `http-and-routing.md`: `resp.AssertInertia("X")`; CSRF section (header
  vs origin trust, how to curl); Inertia form semantics (409 +
  `X-Inertia-Location`, 200 with `errors`, version header).
- AGENTS.md template: skill paths are harness-relative (not only
  `.claude/`); `andurel skill show <reference>` prints references.
- `layer-placement.md`: row for read-only embedded content/static data;
  CSS entry points and which `@theme` owns tokens; frontend section
  (pages, layouts, components, shared props, `RootData`).
- New patterns: `middleware`, `test-middleware`, `page` (Inertia page +
  props + layout).
- Framework README: remove `handlers/api/`, `handlers/auth/`, `--api`.

### 3.8 CLI help/metadata drift
- `sync routes`, `sync payloads`, `tool sync|list|set-version`,
  `inspect project`: `andurel.toml` vs `andurel.lock` split stated
  correctly. Document the split in `generated-files-and-upgrades.md`.
- `db console --help`: `database migrate` → `db migrate`.
- `db seed --help`: drop "model factories"/"v1"; say which seed set is
  default; same for `db rebuild --seed`.
- `generate --help`: point to `andurel patterns show <kind>`.
- `commands --json`: `short` matches each command's help; subcommands get
  `meta` (mutating, supports_dry_run).
- `db` group help notes which commands support `--json`.
- `--jq`: support array index (`.routes[0].name`) and fix the misleading
  hint.
- `inspect routes`: always emit `host` (primary host name).
- `inspect jobs`: classify by content (args type vs `river.Worker`), not
  file name.
- Install command: one source of truth (`andurelInstallCommand`) used by
  README, scaffold Welcome page and doctor.
- Node requirement: `engines` in `package.json` templates; README
  prerequisites.

### 3.9 Scaffold polish
- `pkg/anduin`: `Blob(status, contentType, []byte)` response helper; an
  exported ETag/304 helper shared with `StaticFS`; `SitemapURL.LastMod`.
- `pkg/anduin`: gzip compression middleware that wraps the outermost
  handler (so ErrorHandler output is included), wired in the scaffold
  behind `HTTP_COMPRESS` (default on). Alternatively add a
  post-ErrorHandler hook so a normal `rt.Use` middleware works.
- `anduintest.New`: `WithErrorHandler` option; scaffold test renderer
  mirrors `newRenderer` (flash + shared providers) via a shared helper.
- Flash wiring: scaffold `newRenderer` installs `kiks` as the flash
  provider; define the flash payload type and generate it for TS; layout
  templates render toasts.
- Theme: one `@theme` block (move duplicate mappings out of `css_base`),
  `html` uses `font-sans`; document tokens usable via `var()`.
- Analytics/telemetry IDs: not scaffold scope; skip.
- Sign-out no-JS fallback: add `POST` sign-out route alongside `DELETE`.

---

## Phase 4 — Ideas (not scheduled)
- Content-drift check for embedded content (an `andurel verify` hook for
  app-defined checks, e.g. run `go test` on tagged packages).
- Docs claim verification (resolve package identifiers/commands in docs).
- `inspect cmd` / process model listing (partly covered by 2.8).
- `andurel inspect routes --public` for parity/sitemaps.

---

## Out of scope (site/orchestrator issues, not framework)
- Port assignment collisions between parallel agents, shared
  `assets/dist`, SSR port visibility in `ps` (orchestration; could be
  helped by `INERTIA_SSR_PORT` in docs only).
- Old site (`bak-andurel-site`) run instructions and dead auth routes.
- Brief contradictions (title separator, registration flash copy, merged
  header), stale "latest" docs in andurel-site content, no browser on
  the machine, merged friction-log lag, report index, definition of done
  for report-only tasks.
- `go run ./cmd/ssr` outside the module (normal Go behaviour).
- `DB_NAME`/CSRF cookie changes vs the v1 site (belongs in the v1→v2
  migration notes; add one line there).

---

## Suggested order / PRs
1. PR: 1.1 + 1.2 (email sender + logging) — small, high value.
2. PR: 1.5 (`pkg/kiks` flash) + flash wiring from 3.9.
3. PR: 1.3 (exact routes + not-found) across `pkg/routing`, `pkg/anduin`,
   CLI generators, scaffold.
4. PR: 1.4 + 2.8 (build all processes, artifact manifest, deploy docs).
5. PR: 2.1–2.7 (SSR defaults, bundle, doctor, root, config split).
6. PR: 3.1–3.3 (verify: frontend, scoped, email drift).
7. PRs: 3.4–3.9 in small batches; help/skill drift can go together.

Each PR: goldens updated, `andurel verify` clean on a fresh scaffold,
CHANGELOG entries for touched `pkg/*` modules.

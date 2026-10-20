# v2 alpha readiness review — scaffolded `exploration` project

Reviewed: a fresh scaffold from `dev-andurel` (built at `a22e5e38`), all hand-written
files, plus `doctor`, `verify`, `sync routes|payloads|queries`, `inspect`, and
`patterns show handler|domain|service|composition`.

## Agent-first readiness score vs v1 (master)

v1 scored from master templates/AGENTS/command set (lighter review); v2 fully reviewed.

| Dimension | v1 | v2 today | v2 after must-fix |
|---|---|---|---|
| Code legibility (no Fx/reflection; net/http; one composition root) | 4 | 9 | 9 |
| Placement certainty (flat domains + colocated SQL, flat handlers, layer rules) | 5 | 8 | 9 |
| Surface area (Inertia only, no factories/generators, ~58k LOC smaller) | 4 | 8 | 8 |
| Discovery (`commands`/`inspect`/`doctor` structured) | 7 | 8 | 8 |
| Guardrails & feedback (`verify` layer lints with `fix`, typed exit codes) | 4 | 6 | 9 |
| Generated-code integrity (`sync queries` no-op, payloads 0 types, routes.ts drift) | 6 | 3 | 8 |
| Docs consistency (AGENTS/README/patterns vs code) | 5 | 5 | 8 |
| Reference patterns vs generators | 5 | 8 | 9 |
| Scaffold safety (auth/security) | ~6 | 5 | 8 |
| Testability (no globals, TestCluster, anduintest, runtime.New) | 5 | 8 | 8 |
| **Overall** | **~5.0** | **~6.8** | **~8.5** |

---

## Must fix before alpha

### M1. Fresh scaffold fails `andurel verify` / `doctor`
- `routes.ts` template drifts from the generator (missing `/** Call once at boot with Inertia shared hosts prop. */`).
- `resources/js/types/payloads.ts` is not scaffolded.
- Fix: emit both via the generators at `new` time; add a golden test asserting `verify` passes on a fresh `new`.

### M2. `sync queries` is a no-op; doctor narsilc drift check disabled
- `dev-andurel` is built against published `pkg/storage v0.8.1` where `QueriesDir = "models/queries"`, so it reports "no annotated narsilc query files found under domains/" despite `domains/user.sql` and `domains/token.sql`.
- Local `pkg/storage` is correct (`domains`); `go.work` isn't used for the installed binary.
- Fix: tag `pkg/*` first, then bump CLI `go.mod`. Better: CLI shouldn't depend on `pkg/storage` for layout paths (own the constants in the CLI/layout).

### M3. `sync payloads` produces 0 types
- Generator only picks up named `XPayload` structs; scaffold `handlers/auth.go` and `patterns show handler` use anonymous `var payload struct{…}`.
- AGENTS.md guidance ("after changing handler payload … run sync payloads") is therefore a no-op; frontend `useForm` keys (e.g. `resetPasswordToken`) are untyped against Go.
- Fix: named payloads (`LoginPayload`, `RegistrationPayload`, …) in scaffold and in the handler pattern.

### M4. Rate limiter is wrong in production — `middleware/auth.go`
- Single `ratelimit:<ip>` key shared by login + registration + confirmation (5 combined per 10 min); successful requests count too.
- Uses `RemoteAddr` → behind a proxy everyone shares one bucket. `pkg/anduin` has no trusted-proxy / real-IP support.
- Over-limit response is `429` with the redirect URL as a text body (`redirectURL` param is misleading).
- `PasswordCreate` (reset request) and `PasswordUpdate` are not rate-limited.
- Fix: key by route name + IP; add trusted-proxy config in anduin; proper 429 (Inertia-friendly error/validation); limit reset endpoints.

### M5. Secrets persisted in job table
- `services/registration.go` / `services/reset_password.go` render emails at enqueue time → plaintext verification codes and reset URLs stored in `river_job.args` for the retention period.
- Fix: enqueue ids/tokens and render in the worker (or encrypt args).

### M6. Password reset flow — `services/reset_password.go`
- Looks up user by email in token metadata instead of `token.UserID` (breaks after email change).
- Old reset tokens never revoked: request uses `Create` (not replace); successful reset doesn't `DestroyByUserAndScope`.
- No index on `tokens(scope, hash)` → seq scan on every lookup.

---

## Should fix

> Status (2026-10-20): S1–S13, README drift and the nits below are resolved in
> pkg/storage v0.10.0, pkg/cache v0.2.0, pkg/anduin v0.2.0, pkg/inertia v0.8.0
> and the scaffold that pins them. S13's River drift was real: River v0.40 added
> main-line migration 7, which is now vendored, and
> `TestRiverMigrationsMatchPinnedRiver` fails when the vendored migrations stop
> matching the pinned River driver.

### S1. Dead / duplicated code
- `middleware.CSRFMiddleware`, `mayBypassCSRF`, `TraceRouteAttributes` are unused; `internal/runtime` re-implements CSRF + bypass with a *different* token lookup (`cookie:` vs `header:`); `middleware_test.go` tests the dead copy.
- `startInBackground` / `stopAndWait` only exercised by `internal/runtime/server_test.go`.
- Queue worker `Register` methods (`AddWorkerSafely`) unused; `NewWorkers` uses `AddWorker`.
- Templ-era asset routes: `Stylesheet`, `Scripts`, `Script`, empty `assets/css/style.css`, 1-byte `assets/js/scripts.js`.

### S2. Login / error UX
- Invalid credentials redirect to sign-in with no message; logged at Error level.
- Decode failures render `Errors/BadRequest` with status 200; the handler pattern says `return anduin.BadRequest(err)`.

### S3. Home page / frontend
- `/` is `AuthOnly`, but Welcome shows "Create account"/login links.
- `Layout.tsx` always shows Log in / Register; no sign-out anywhere.
- Internal links use `<a>` instead of `<Link>`; nested `<main>` in Welcome; duplicated background gradient.
- Welcome copy mentions "controllers", "Generate models, factories, controllers…".

### S4. Asset cache-busting via process start time
- `routes.startTime` differs per replica → 404s on multi-instance / rolling deploys. Vite output is already content-hashed; drop the timestamp segment (or use build version).

### S5. Two transaction APIs in `pkg/storage`
- `RunInTransaction`/`Transaction` and `AtomicOps`/`Tx` coexist; domain pattern rules still mention the old pair. Pick one before alpha.

### S6. `Users.Update` zero-value traps — `domains/user.go`
- Omitting `IsAdmin` resets it to false; `EmailValidatedAt` cannot be cleared; `Password` accepts a raw hash (bypasses hashing); `ChangePassword` reads twice.

### S7. Token surface — `domains/token.go`
- `Tokens.All` / `Tokens.Paginate` expose hashes and are unused.

### S8. Copy mismatches
- Verify email says "expires in 15 minutes"; code sets 24h.
- `ConfirmEmail.tsx` says "6-digit"; code is alphanumeric.

### S9. PII in telemetry
- `user.email` logged on registration; `email.to` on worker spans.

### S10. Config
- `config.NewApp` calls `routing.ConfigureHosts` (global side effect in a constructor).
- `CacheTTL()` reads env outside the config struct.
- `DefaultDatabaseName`, `DefaultDatabaseApplicationName`, `DefaultCacheNamespace` all `"andurel"`; `.env.example` `DB_NAME=andurel` — should be project-derived.
- `config/hosts.go` is an empty file (add a commented example).

### S11. `.env.example`
- Missing `ENVIRONMENT`, `PORT`, `MAILPIT_*`, telemetry vars the README lists.

### S12. Runtime wiring — `internal/runtime/runtime.go`
- Startup error cleanup (`_ = db.Close(); _ = tel.Shutdown(ctx)`) copy-pasted 5×; append to `closers` as you go.
- Cache store never closed (Redis client leak); `pkg/cache` has no `Close`.
- `anduin.NewServer(ctx, host, port, env, handler, nil, …)` then `Start(ctx, env)` again.
- `inertia.NewRenderer` / `inertia.NewSSRRuntime` take 7 positional args.

### S13. Migrations
- Goose template noise (`SELECT 'up SQL query';`) and `CREATE TABLE IF NOT EXISTS` (hides drift).
- River schema vendored as goose migrations 00001–00006 — will drift on River upgrades.

---

## README drift
- Go 1.24.4 vs `go.mod` 1.27.2.
- Tree lists `bin/`, `queue/workers/`, `internal/clients|payment/` (none exist).
- Handler example: `func (h Handler)`, `product.NewProducts`, `producthandlers.NewHandler`.
- Queue section is goqite-style (`ProcessMyJob(ctx, msg []byte)`, `queue.Enqueue`, "Register in your queue setup" placeholders); Learn More links goqite.
- `verify` description differs from AGENTS.md.

---

## Nits
- `middleware.App` is a third `App` type — rename `Session`.
- Duplicate route names `users.user_password` (Create/Update); routing doesn't detect duplicate names.
- `GenerateCode` modulo bias (256 % 36).
- `ValidPassword` error says "decode pepper" (it's the salt).
- Mixed `%v` / `%w` wrapping in services.
- Development seed not idempotent; `"default"` duplicates `"development"`.
- `handlers/assets.go` caches ETags in the cache store for immutable embedded content — compute once at startup.
- Sitemap hardcoded `LastMod: 2024-10-22`; unused `[]routing.Route` param; exported `URL`/`Sitemap` types in `handlers`.
- `root.templ` hardcodes `class="dark"`.

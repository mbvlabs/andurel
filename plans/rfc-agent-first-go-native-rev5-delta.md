# RFC rev-5 delta — changes under discussion only

Against `plans/rfc-agent-first-go-native.md` rev 3. No unchanged RFC text repeated.

## 0. `domains/` is one flat package (rev 6)

- No `domains/user/`, `domains/token/` sub-packages: `domains/user.go`, `domains/token.go`, `domains/user.sql`, all `package domains`.
- Rationale: same as handlers — sub-packages forced `domainuser`/`domaintoken` aliases at every service, handler, seed, and composition call site. Domains never use each other in practice (services coordinate), so nothing is lost: what stays mechanically enforced is the queries-only import lint (`verify` layers). One file per domain is convention.
- `domains/internal/queries` stays a separate package; only `domains` imports it.
- `inspect domains` reports per-file entities; `verify` keeps the queries-only check.

## 1. `handlers/` is one flat package

- Delete `handlers/user/`, `handlers/auth/`, `handlers/api/` sub-packages.
- One `package handlers` with `NewUserHandler`, `NewAuthHandler`, `NewAPIHandler`, `NewPagesHandler`, `NewAssetsHandler`, each with `Register(rt *anduin.Router)`.
- Rationale: sub-packages force a triple import in `runtime.go` plus the `domainuser` alias dance (§4.3) for zero isolation — handlers share renderer, error mapper, router type.
- Non-handler code (`Cache[T]` builder in `handlers/cache.go`) moves out to `internal/runtime` or its owning package.

## 2. No `middleware.NewJar`

- `middleware/` exports policy only: auth guard, `IPRateLimiter`, session `Definition` (`NewAppCookie`).
- Jar construction (`kiks.NewJar` + `NewAppCookie`) moves to `internal/runtime.newJar`. `middleware.NewJar` is deleted.
- Thin re-export wrappers (`Telemetry`, `Logger`, `CSRFMiddleware` over `anduin.*`) are deleted; app uses `pkg/anduin` directly.

## 3. Single-constructor `runtime.New`

- `internal/runtime/server.go` (`NewServer`) is merged away. `New(ctx, cfg)` builds telemetry → DB → domains → services → renderer → jar → router → `anduin.Server` and returns `App{Config, DB, Handler, Server}`.
- `App` gains `Start`/`Shutdown` delegating to `Server`; `App.Close` runs closers in reverse. `cmd/app` becomes env load + `runtime.New` + `app.Start`.
- `pkg/anduin` still keeps `Router` and `Server` as separate types (handler tests use `Router` alone); only the app stops exposing two constructors.

## 4. Type-driven domains (Palmieri ch. 6: parse, don't validate)

- Each invariant becomes a newtype with `ParseX` as the supported constructor; read access is read-only (`String()`, `Bytes()`).
- Example: `type Email string` + `ParseEmail(raw) (Email, error)` (trim, lowercase, format check, `ValidationError` on failure). `User.Email` is `Email`, not `string`. `CreateUserData` takes parsed types.
- The exported `Validate() *User` post-construction pattern is removed. Unit tests target `Parse` directly.
- CLI carries it: `patterns show domain` embeds the newtype/`Parse` worked example plus a checklist item ("all entity inputs are parsed types; no post-construction `Validate`"); the `--entity/--table` builder emits newtypes + `Parse` constructors + `Parse` unit tests, not raw-string structs; `inspect domains --json` reports the type→column mapping including newtype coverage.

## 5. `verify` vs `doctor`: split, no merge

- `verify` = write-path gate (gofmt, `go fix`, `go vet`, layers lint, `--with-tests`). Fails CI.
- `doctor` = read-only diagnostics (env, lock, tools, tidy, templ/narsilc/routes drift). Warns only.
- Shared `go vet` helper to dedupe; `layers` check optionally surfaces in `doctor` as a warning.

## 6. `patterns show domain --entity/--table`

- The old `generate model User` convention survives only as builder output: `patterns show domain --entity User --table users` emits the ordered file manifest (migration + colocated `.sql` + newtype entity/receiver + test) to stdout/Markdown/`--json`. Patterns still never write files.
- `generate` keeps only `migration`. `inspect domains --json` reports the type→column mapping.

## 7. Open items (not yet in RFC text)

- **Queue: parked.** Keep River as-is for now — no facade, no replacement. Revisit post-rev-5; code is cheap enough that a later move off River (keeping transactional enqueue, retries, periodic) stays on the table.
- **Cache: new `pkg/cache` package (not `storage`).** Storage owns Postgres/narsilc/queue; cache is a separate concern with memory + Redis drivers and deserves its own package. Laravel-inspired but Go-native concrete surface:
  ```go
  type Store interface {
      Get[T any](ctx context.Context, key string) (T, bool)
      Put(ctx context.Context, key string, val any, ttl time.Duration) error
      Remember[T any](ctx context.Context, key string, ttl time.Duration, loader func() (T, error)) (T, error)
      Forget(ctx context.Context, key string) error
      Flush(ctx context.Context) error
      Increment(ctx context.Context, key string, window time.Duration) (int64, error)
  }
  ```
  Drivers: `memory` (otter + singleflight) and `redis`. Built once in `runtime.newCache` from `config.Cache{Driver}`, injected into handlers/middleware — replacing the hand-rolled otter in `handlers/cache.go` and `middleware/auth.go`.

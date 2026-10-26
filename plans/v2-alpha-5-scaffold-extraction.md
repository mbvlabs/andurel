# RFC: Shrink the v2-alpha.5 scaffold to application code

Source: `../v2-alpha-5` (scaffolded at `f5c6c20e`). About 4,500 hand-written Go lines, not counting generated `*_templ.go` and `queries/`.

## Principle

A scaffolded file should hold one of two things:

1. **Application code**: entities, business rules, use cases, routes, pages, and session shape. The user owns it and is expected to change it.
2. **Explicit wiring**: short calls into `pkg/*` that pass application values in. Users can read and reorder it, but it contains no algorithms.

Anything else (crypto, env parsing, HTTP plumbing, error translation, lifecycle handling) is framework code that has ended up in user space. Every copy of it is a signature `andurel upgrade` has to patch, and a security fix we can't ship with `go get`.

Rough estimate: **~1,200 lines (~25–30%) of the scaffold can move into `pkg/`**, plus ~300 test lines that go with it. The `config/` sections stay as they are.

---

## 1. Domains: validation and the public/private split

### What's there

| Symbol | Visibility | Why it exists |
|---|---|---|
| `User.Validate()` | public | Wraps `validate(b)` |
| `User.validate(b)` | private | Lets `Create` put entity and password errors in one builder |
| `PasswordPair.validate(b)` | private | Same reason |
| `Token.Validate()` | public | Builder inline, so it's already inconsistent with User |
| `validation.Validate(&entity)` | — | Indirection via `any` → `Validatable`, used in `save`/`createToken` instead of `entity.Validate()` |
| `validEmailAddress` | private | A generic rule living in the domain |
| `save`, `createToken`, `createTokenData`, `normaliseEmail`, `userWriteError`, `generateSalt` | private | Mixed: half are framework code, half are domain operations hidden for no reason |

The two-method pattern only exists because `ValidationBuilder` is the unit of composition. If `error` is the unit of composition instead, it goes away.

### Proposal

**Framework (`pkg/validation`):**

- `validation.Join(errs ...error) error`: flattens any `ValidationErrors` into one `ValidationErrors`, drops nils, and `errors.Join`s anything that isn't a validation error onto the result.
- `b.Email(field, value, msg...)`: replaces `validEmailAddress`.
- `validation.Field(field, code, msg) error`: a one-error shortcut, which makes `invalidVerificationCode()` a single line.
- Deprecate `validation.Validate(any)` / `Validatable` once the scaffold stops using them. Calling `x.Validate()` directly is clearer.

**Scaffold rule:** each type has exactly one method, the public `Validate() error`. No `validate(b)`.

```go
func (u User) Validate() error {
	b := validation.NewBuilder()
	b.Required("email", u.Email)
	b.MaxLen("email", u.Email, 255)
	b.Email("email", u.Email)
	return b.Err()
}

func (p PasswordPair) Validate() error { /* same shape */ }

// Users.Create
if err := validation.Join(entity.Validate(), data.PasswordPair.Validate()); err != nil {
	return User{}, err
}
```

**Make it public by default:** `save` → `Users.Save`, `createToken`/`createTokenData` → `Tokens.Create(ctx, CreateTokenData)` (the current `Create`, which mints a secure token, becomes `CreateSecure`), and `normaliseEmail` → `NormaliseEmail`. Once the framework helpers below exist, the domain has nothing left to hide. Then `services` can call `Users.Save` and `Tokens.Create` directly, and it no longer needs a new domain method for each variation.

### Domain code that is framework code

| Today (domain) | Move to | Notes |
|---|---|---|
| `HashPassword`, `User.ValidPassword`, `generateSalt` (argon2id + pepper, ~60 lines) | **new `pkg/auth`**: `auth.HashPassword(pw, pepper)`, `auth.VerifyPassword(hash, pw, pepper)` | Security-critical. The hash format must be versioned in one place. |
| `services.verifyPasswordWithPeppers` and the pepper trimming in `NewIdentity` | `auth.VerifyPasswordAny(hash, pw, current, previous...) (ok, needsRehash, err)` and `auth.Peppers(current, previous)` | Its table test moves with it. |
| `GenerateCode`, `GenerateSecureToken`, `HashForStorage`, `Token.IsValid` (HMAC compare) | `pkg/auth`: `auth.GenerateCode(n)`, `auth.GenerateToken()`, `auth.HashToken(plain, secret)`, `auth.VerifyToken(plain, secret, hash)` | `Token.IsValid` becomes `auth.VerifyToken(plain, secret, t.Hash) && t.ExpiresAt.Time.After(now)`. |
| `if errors.Is(err, pgx.ErrNoRows) { return X{}, ErrNotFound }` ×7 | `storage.Row(v, err)` generic, which maps no-rows to `storage.ErrNotFound` | `Find` becomes `return storage.Row(u.queries.GetUser[User](ctx, id))`. See "Domain sentinels" below. |
| `userWriteError` (pg `23505` + constraint name) | `storage.Conflict(err, "users_email_key")`, which wraps `storage.ErrConflict` | Removes the `pgconn` import from domains. |
| `Paginate` clamping, offset and totalPages; `PaginatedUsers` | `storage.Page(page, size) (limit, offset)` + `storage.Paginated[T]` | Each future entity would otherwise copy ~40 lines. |

### Domain sentinels

Today every domain method converts `pgx.ErrNoRows` into `domains.ErrNotFound`, and `handlers/errors.go`, services and seeds all check `errors.Is(err, domains.ErrNotFound)`. Once `storage.Row` does that conversion, the error that comes out is `storage.ErrNotFound`. Callers would still check `domains.ErrNotFound`, so the two need to be the same value:

```go
// domains/errors.go
var (
	ErrNotFound = storage.ErrNotFound
	ErrConflict = storage.ErrConflict
)
```

Nothing above `domains` changes: handlers and services keep writing `domains.ErrNotFound`, and the conversion lives in one place in the framework.

What's left in `domains/user.go`: the entity, `Validate`, `HasValidatedEmail`, the CRUD methods (each 3–10 lines), `PasswordPair`, and `NormaliseEmail`. That's roughly 403 → 180 lines. `token.go` goes from 335 → ~150.

---

## 2. Config: only the env reader moves

The config sections (`App`, `HTTP`, `Auth`, `Session`, `Cache`, `Database`, `QueueInsert`/`QueueWorker`, `Telemetry`, `Mail`, `Inertia`) **stay as they are**, in the app, with the same fields, constructors and validation.

The only thing that moves is `config/helper.go` (172 lines). It's a general env reader (`String`, `RequiredString`, `Int`, `Int32`, `Int64`, `Bool`, `Float64`, `Duration`, `Strings`, `Err`) plus `LoadEnvironment`. None of it is specific to the application.

### Proposal: `pkg/env`, with no dependencies

- `env.Load(files...) error` replaces `config.LoadEnvironment`. **We parse the `.env` file ourselves and drop godotenv.** The format we generate is simple: `KEY=VALUE`, `#` comments and blank lines (see `.env.example`). Supporting that, plus optional `export ` prefixes and single/double-quoted values, takes ~60 lines. It also removes a dependency from every scaffolded app. As today, keys that are already set in the process environment win, and a missing default `.env` is not an error.
- `env.New()` returns the reader. `env.NewWithLookup(fn)` is for tests (today's `newEnvironmentWithLookup`). The methods stay the same as today's, just exported.
- Section files change one line each: `env := newEnvironment()` becomes `e := env.New()`. Nothing else in `config/` changes.

Wrapping godotenv wouldn't be worth a package. A zero-dependency reader with our own loader is what makes `pkg/env` worth having.

---

## 3. HTTP layer

| Today | Move to | App keeps |
|---|---|---|
| `handlers/assets.go` (220): ETag walk, immutable caching, 304, path cleaning, content types, `writeContent`, robots, sitemap XML | `anduin.StaticFS(fsys, anduin.Immutable(prod))`, `anduin.RobotsTxt(sitemapURL)`, `anduin.Sitemap(urls...)`; content types via `mime.TypeByExtension` | ~20 lines: three `rt.Get` calls and the sitemap URL list |
| `handlers/errors.go` (75): JSON-vs-Inertia detection, validation → 422, status mapping, 5xx passthrough | `inertia.ErrorHandler(renderer, inertia.MapError(domains.ErrNotFound, 404, "Errors/NotFound"), ...)`. Inertia detects field errors through an `interface{ ToMap() map[string]string }`, so it needs no validation dependency. | The map from sentinels to pages |
| `handlers/api.go` Health | `anduin.Health(checker)` handler | Route registration |
| `validationFields(err)` + `ValidationErrors(fields).Render()` ×6 in auth | `page.Errors(err)`, which takes an `error`, pulls out the field map, and reports whether it handled it | Handler flow |
| `middleware.RateLimit` (+157 test lines) | `anduin.RateLimit(counter, limit, window, onLimited)` with `counter interface{ Increment(ctx, key, ttl) (int64, error) }`. `cache.Store` already satisfies it. | `throttle` and the 429 page |
| `middleware.CSRFSkipper` / `mayBypassCSRF` / `isAPIPath` / `hasNonEmptyBearerToken` (+78 test lines) | `anduin.SkipCSRFForBearer(apiPrefix, sessionCookie)` | — |
| `runtime.corsConfig` / `csrfConfig` / `mergedOrigins` (~70 lines) | `anduin.DefaultCORS(origins...)`, `anduin.DefaultCSRF(strategy, secure, origins...)`, `anduin.MergeOrigins` | Calls that pass origins |
| `Session.MarshalCookie` / `UnmarshalCookie` (JSON boilerplate) | `kiks` falls back to JSON when `T` does not implement `Cookie` | The `Session` struct itself and `NewSessionCookie` |

`AuthOnly`, `setPendingUser`, `pendingUserID`, `sharedAuth` and all of `handlers/auth.go`'s flow stay as application code.

---

## 4. Email and queue

- `email/email.go` (`Transformer` interface + an `HTMLToText` passthrough) → `pkg/email.Transformer`. The emailcompiler emits `email.Transformer` / `email.HTMLToText` from `pkg/email` directly, and the app file is deleted.
- Render-to-`TransactionalData` is repeated in `VerificationEmail` and `PasswordResetEmail` → `email.Compose(t Transformer, to, from, subject) (TransactionalData, error)`.
- `SendTransactionalEmailWorker`, `SendMarketingEmailWorker`, their `jobs.*Args`, and `sendTransactional` (cancel on non-retryable) are generic → **`pkg/storage`**, which already owns river: `storage.AddEmailWorkers(workers, transactional, marketing)`, plus the `storage.SendTransactionalEmailArgs` / `SendMarketingEmailArgs` job types. This makes `pkg/storage` depend on `pkg/email`, which only carries the sender interfaces and data types. It does not go the other way.
- The verification and reset workers stay. They are application use cases.
- Smell to fix while there: `runtime.NewQueue` builds an insert queue it never uses, only to satisfy `NewIdentity`. Split `Identity` or accept a nil queue.

---

## 5. Runtime, cmd, tests

- The closer stack (`closers` + `slices.Backward` + `errors.Join`) appears twice → new `pkg/lifecycle`: `lifecycle.Stack` with `Add` and `Close(ctx)`.
- The signal/serve/10-second-shutdown boilerplate appears three times in `cmd/app`, `cmd/queue` and `cmd/ssr` → `lifecycle.Run(ctx, start, stop)`. Each `main` drops to about 15 lines.
- `storage.WithOpenTelemetry` takes an interface `{ TracerProvider(); MeterProvider() }`, so the app passes `tel` directly and `runtime.newDatabase` goes away.
- `TestMain` + `testCluster` is copied three times (domains, handlers, runtime) → `storagetest.Main(m, &testCluster)`.
- `seeds` registry (`Registry` / `Names` / `Run`, ~35 lines): low value. Leave it.

The router middleware stack in `runtime.New` **stays explicit**. Users need to see and reorder it.

---

## 6. Services (application code, light touch)

- `LoginData`, `RequestResetPasswordData` and `VerifyEmailData` do `Required` checks inline in the service, which contradicts the package doc ("entity validation belongs to domains"). Give them `Validate()` methods. They're input types, so the same rule applies.
- `PasswordResetEmail` does destroy + create inline, while verification uses `Tokens.ReplaceCode`. Add `Tokens.Replace` (secure variant) for symmetry. This gets simpler once `Tokens.Create(ctx, CreateTokenData)` is public.
- Optional: inject `pepper` / `secret` into `Users` / `Tokens` at construction instead of passing them on every call. That's 6 fewer parameters on public domain methods.

---

## Order of work (all in alpha.5)

Everything here ships in alpha.5:

1. `validation.Join`, `b.Email`, `validation.Field`; collapse domain `validate(b)` into public `Validate()` and make the private domain helpers public
2. `pkg/auth` (passwords, pepper rotation, tokens/codes); `storage.Row` / `Conflict` / `Page`; alias the domain sentinels
3. `anduin.StaticFS`, `RateLimit`, `SkipCSRFForBearer`, `Health`; `inertia.ErrorHandler`, `page.Errors(err)`
4. kiks JSON fallback; `email.Transformer` / `Compose`; `storagetest.Main`
5. `pkg/env` (own `.env` loader, godotenv dropped); `config/helper.go` deleted
6. Email workers into `pkg/storage`; `pkg/lifecycle`

Each `pkg/` change needs its own CHANGELOG entry, and the layout templates and golden files must be regenerated.

## New modules

`pkg/auth`, `pkg/env`, `pkg/lifecycle`, each its own module. None depends on another andurel package.

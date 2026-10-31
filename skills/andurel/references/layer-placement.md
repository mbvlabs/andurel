# Layer Placement

Rules for placing code in an Andurel app. The shape is fixed and deliberately
small. Generated applications have no top-level `internal/`: an application is
not a library, so packages are top-level and named for their job. The one
exception is `domains/internal/queries/`, which the compiler keeps private to
`domains` because only domains may read it.

- HTTP is ordinary Go over `net/http`, reached through
  `github.com/mbvlabs/andurel/pkg/anduin` (see
  [http-and-routing.md](http-and-routing.md)).
- Business logic and persistence live in `domains/`.
- Application logic (cross-domain orchestration) lives in `services/`.
- HTTP adapters live in `handlers/`; route data lives in `routes/`; app
  middleware live in `middleware/`.
- Inertia page props and request bodies live in `payloads/`.
- Composition is explicit constructor injection in `app/`. There is no Fx, no
  reflection wiring, and no container.

Two commands keep an agent honest:

- `andurel patterns show <kind>` prints reference code, the rules for that
  layer, and a "definition of done" checklist. Patterns are read-only; the agent
  writes every file.
- `andurel verify` runs formatting, `go fix`, `go vet`, the typed layer rules
  that enforce the boundaries below, and generated-code drift. The rule list is
  in [cli-catalog.md](cli-catalog.md#verify-rules). Add `--with-tests` to also
  run `go test ./...`.

## Placement Rules

| Code | Put it in |
| --- | --- |
| Database-backed entity structs, `andurel:"column"` table tags, create/update input structs, validation, persistence methods, finder/query methods, domain errors | `domains/` (for example `domains/user.go`) |
| SQL for a domain (narsilc input) | `domains/<entity>.sql`, colocated with the entity |
| narsilc-generated query types and rows (narsilc output) | `domains/internal/queries/` — imported only by `domains/**` |
| Cross-domain vocabulary: `ErrNotFound`, `ErrConflict` | `domains/errors.go` (package `domains`) |
| Rules that must hold regardless of HTTP, Inertia, API, jobs, or seeds | `domains/` |
| Multi-step workflows, transactions, cross-domain coordination, external side effects, application policy that requires current database state | `services/` |
| Request parsing, path/query/form/JSON binding, HTTP status/render/redirect choices, flash messages, CSRF/session handling | `handlers/` (one flat package) |
| Inertia page props and request body structs emitted to TypeScript | `payloads/` (package `payloads`; standard library, `time`, and `uuid` only) |
| Route names, route paths, URL builders | `routes/` |
| App HTTP middleware (auth, authorization) | `middleware/` |
| Inertia root document | `resources/root.templ` (package `resources`) |
| Inertia pages and reusable frontend components | `resources/js/Pages/`, `resources/js/Layouts/`, `resources/js/Components/`, `resources/js/components/ui/` |
| Generated TS route/payload helpers | `resources/js/routes.ts`, `resources/js/types/payloads.ts` (written by `andurel sync routes` / `sync payloads`) |
| Tailwind entry point | `css/base.css` (imports Tailwind and `css/theme.css`; `@source` globs live here) |
| Theme tokens and their utility mappings | `css/theme.css` (`:root` values plus the single `@theme` color owner) |
| Compiled/static assets | `assets/` |
| Read-only embedded content or static data consumed by Go | A top-level package named for its job (for example `content/`) with `//go:embed`; not `assets/`, which is served, or `resources/`, which is frontend |
| SQL schema changes | `migrations/` |
| Apply pending SQL (one-shot process) | `cmd/migrate/` |
| Seed data | `seeds/` or `cmd/seeds/` following the existing pattern |
| External provider adapters | A top-level package named for its job (for example `clients/`, `payment/`) |
| Email templates and send helpers | `email/` |
| River job argument types | `queue/jobs/` (`pkg/storage` owns the generic email args) |
| River workers and queue registration | `queue/` |
| App configuration structs and environment loading | `config/` |
| Process composition (constructor wiring, infrastructure, process lifecycle) | `app/` |
| Process entry (load env, one `lifecycle.Run` around `app.New` / `app.NewQueue`) | `cmd/app/`, `cmd/queue/` |
| One-shot database processes | `cmd/migrate/`, `cmd/seeds/` |

## Domains

`domains/` is all business logic and persistence in one flat package — one
file per domain (`domains/user.go`, `domains/token.go`). A domain answers
"what is true about this entity?" and owns the SQL that reads and writes it.

Use a domain file for:

- Entity structs. Entity timestamps are plain Go times: `time.Time` for a NOT
  NULL timestamp and `*time.Time` for a nullable one. narsilc scans each tagged
  column into its field directly, so the entity needs no `Transform` for them.
  Keep fields mapped to the table (`andurel:"column"` tags), and keep
  `sql.Scanner` off the entity. `pgtype.Timestamptz` appears only in generated
  query params and rows inside `domains/`; a nullable `*time.Time` converts to
  the pgtype for Create and Update params through an unexported helper. Only
  `timestamptz` is converted that way; other pgtype row types pass through.
  Implement `Transform(queries.XRow) error` only when the mapping genuinely
  differs from the row. A `Transform` entity implements `narsilc.Transformer`,
  so `entity-columns` does not check it and `inspect domains` reports
  `columns_ok: null`; update its `Transform` by hand when the columns change.
  Entities on the automatic mapping are checked: a generated query column with
  no tagged field fails `andurel verify`.
- Generated `queries.XRow` / `queries.QueryNameRow` values. Domain code returns
  the entity, never the row.
- `Create<Entity>Data` and `Update<Entity>Data` input structs.
- `Validate()` methods using `pkg/validation` with field normalization
  (trim, lowercase) and `validation.ValidationErrors` failures.
- CRUD methods, finder methods, pagination, count queries, and domain query
  methods.
- Domain errors. Map expected persistence failures to the shared sentinels
  (`ErrNotFound`, `ErrConflict`); return `pkg/validation` errors directly for
  field-level failures. Preserve unexpected database errors for diagnostics.

Hard rules:

- **One file per domain; keep cross-domain rules in services.** If a rule spans
  `user` and `token`, it is application logic and belongs in a service.
- **Only `domains` touches the database.** Only `domains` imports
  `domains/internal/queries`, and only `domains` (plus `app/`, `cmd/`, `seeds/`,
  and `migrations/` for wiring) may name `storage.Connection` or
  `storage.Transaction` or import pgx, and only `domains` may call
  `storage.Unwrap`. Everyone else receives domain receivers
  (`domains.NewUsers(db)`, built in `app`) and, for transactions, a
  `storage.Transactor`. `andurel verify` enforces this.
- **Every receiver has `New<Plural>(db)` and `InTx(tx storage.Tx)`.** A receiver
  holds a `storage.Transactor` when one of its own methods must be atomic by
  itself, either because it writes more than once or because it reads, modifies
  and writes a row. It rebinds the transactor in `InTx`, wraps the statements in
  `storage.AtomicOps` so it joins a caller's transaction when bound, and locks
  the row with `FindForUpdate` before a read-modify-write. `Save` writes every
  column and leaves locking to a caller that uses it directly.
- **Validate once, in the domain.** `Create`/`Update` call the entity's
  `Validate()` before writing and return `pkg/validation` errors directly.
  Handlers and services do not repeat entity rules.
- **SQL is colocated with its domain** (`domains/user.sql`). Narsilc globs
  `domains/**/*.sql` and writes to `domains/internal/queries/`.
- **The Go entity is the source of truth.** Inspect the type→column mapping,
  `Validate()` coverage, and wiring with `andurel inspect domains --json`.
- Keep domain methods independent of HTTP, `anduin`, Inertia, sessions, cookies,
  flash messages, and route names.

## Services

Create or extend a service when the use case is bigger than one domain
operation: transactions spanning domains, cross-domain coordination, workflow
policies that need current database state, or coordinating persistence with
email, queue insertion, cache invalidation, or external clients.

A service declares its dependencies individually as constructor arguments:

```go
func NewIdentity(
	transactor storage.Transactor,
	users domains.Users,
	tokens domains.Tokens,
	queue storage.InsertQueue,
	authCfg config.Auth,
	mailCfg config.Mail,
) Identity
```

Writes that must commit together run inside `storage.AtomicOps`. Bind what the
use case needs at the top of the callback and use only the bound locals:

```go
err := storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {
	users, queue := i.users.InTx(tx), i.queue.InTx(tx)
	// … users.Create(…); queue.Insert(…) — the job commits with the data …
	return nil
})
```

Returning `nil` commits; returning an error or panicking rolls back. Helpers
take bound dependencies as parameters; public use cases do not call each other
inside a callback. See `andurel patterns show service`.

Do not create a service for a pass-through wrapper around one domain method.
Call the domain directly from the handler until a real workflow appears.

## Handlers

Handlers are thin HTTP adapters: `func (h Handler) Index(ctx context.Context)
error`. They read input with `anduin`, decode request bodies into a struct from
`payloads/`, call a domain or service, and render or redirect.

- `handlers/` is one flat package: distinct `NewXHandler` constructors, no
  sub-packages, no import aliases.
- Page props and request bodies are exported structs in `payloads/`, passed
  with `inertia.FromStruct(payloads.SomeProps{...})` and decoded with
  `anduin.DecodeJSON` / `anduin.DecodeForm`. Do not define them inline.
- Do not put durable business rules in handlers. Return errors unchanged and
  let the router's `ErrorHandler` (`handlers.MapError`) map them. See
  [http-and-routing.md](http-and-routing.md) for the request/response API.

## Payloads

`payloads/` is the single source of the TypeScript payload types that
`resources/js/types/payloads.ts` exports, just as `routes/` is the source of
`routes.ts`.

- Every exported struct in `payloads/` is emitted; nested named structs in the
  same package are followed. `json:"-"` is omitted and `omitempty` makes a
  field optional.
- Keep payload fields to standard-library types, `time`, and
  `github.com/google/uuid`. Copy the fields a page or form needs from a domain
  entity instead of embedding the entity.
- `payloads/` may not import the application module, `pkg/storage`,
  `pkg/anduin`, `pkg/inertia`, or `net/http`; `andurel verify` enforces this
  with the `payloads-import` rule.
- After adding or changing a payload struct, run `andurel sync payloads` and
  import the generated types from `@/types/payloads`.

### Mapping nested read models

Keep the transport shape independent of the read model. For example, declare
the following in `payloads/`, with no import of the application docs package:

```go
type NavItem struct {
    Title string `json:"title"`
    Children []NavItem `json:"children"`
}
type Preview struct { Token string `json:"token"` }
type DocsShowProps struct {
    Navigation []NavItem `json:"navigation"`
    Preview *Preview `json:"preview"`
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

In the handler, explicitly map the read model, sharing the same mapping and
`DocsShowProps` for public and preview pages. Map each field deliberately, so
internal fields cannot leak. For example, if `docs.Node` has the same named
fields, the recursive adapter preserves nil collections rather than silently
turning JSON null into an empty array:

```go
func navPayload(nodes []docs.Node) []payloads.NavItem {
    if nodes == nil { return nil }
    result := make([]payloads.NavItem, 0, len(nodes))
    for _, node := range nodes {
        result = append(result, payloads.NavItem{
            Title: node.Title, Children: navPayload(node.Children),
        })
    }
    return result
}
// Both public and preview handlers use navPayload; set Preview only for preview.
props := payloads.DocsShowProps{Navigation: navPayload(model.Navigation)}
if preview != nil { props.Preview = &payloads.Preview{Token: preview.Token} }
return renderer.Page(ctx, "Docs/Show", inertia.FromStruct(props)).Render()
```

Test the adapter and encoded JSON for public and preview, including nested nil
and empty slices. A nil slice without `omitempty` is `null`, not `[]`; choose
non-nil slices if the wire contract guarantees arrays, otherwise handle null
recursively in the frontend. A nil pointer without `omitempty` emits null;
`omitempty` omits a nil pointer and nil or empty map/slice. A non-nil pointer
to a nil map still emits null. Preserve these distinctions when changing DTOs.
`map[string]any` (also `map[string]interface{}`) represents heterogeneous JSON
objects as `Record<string, unknown> | null`, or an optional non-null record
with `omitempty`; values must actually be JSON-marshalable. Narrow unknown
values before using them in TypeScript. `json.RawMessage` remains useful for
already-encoded arbitrary JSON, including non-object values.

`inertia.FromStruct` can place a typed nil pointer in its props map: comparing
`props["preview"] == nil` is false even though JSON encodes it as null. Root
policies inspecting props before encoding must recognize the expected typed nil
pointer explicitly, and fail closed for unexpected types; test both cases.

## Routes

Route *data* lives in `routes/` as package variables built with the
`github.com/mbvlabs/andurel/pkg/routing` constructors:

- `routing.NewSimpleRoute` for fixed paths.
- `routing.NewRouteWithSlug`, `routing.NewRouteWithSerialID`,
  `routing.NewRouteWithUUIDID`, `routing.NewRouteWithToken`,
  `routing.NewRouteWithFile`, and `routing.NewRouteWithParams` as needed.

Route values expose `Name()`, `Path()`, `Pattern()`, `URL(params...)`,
`FullURL(hosts, ...)`, `Host()`, and `IsInertia()`. Register with
`rt.Route(route).Get(h)`, which uses the route's declared host, and use `Path()`
for links. A route whose path is `/` matches only `/`; a catch-all is `/*`. Pass `routing.TypeScriptRoute()` to opt a route into generated
TypeScript helpers. After adding or changing routes, run
`andurel sync routes --json`.

The `Router` type itself is not app code; it lives in `pkg/anduin`. There is no
`router/` package.

## Middleware

App-specific HTTP policy lives in `middleware/` with the `anduin.Middleware`
shape (`func(anduin.Handler) anduin.Handler`): auth and authorization
(`AuthOnly(users, hosts)`, `ViewerFrom`), the session definition and cookie builder,
and route/telemetry helpers. Generic policy — rate limiting, CSRF, CORS — comes
from `pkg/anduin`.

Cookie jar construction lives in `app/` (`newJar`); `middleware/` exports the
session definition and application policy only. There is no
`middleware.NewJar`.

Session/bag values travel on `context.Context` through `pkg/kiks`
(`kiks.Get[T]`, `kiks.Set`, `kiks.Destroy`). See
[http-and-routing.md](http-and-routing.md).

## Resources And Frontend

`resources/root.templ` is the one page-shaped templ in the app: the Inertia
root document (package `resources`). It is the only templ outside `email/`.
Page data never travels through `RootData`; pass it as props. Shared props are
the exception: `app/` configures `inertia.WithShared` with `appVersion` and
`hosts`, and the scaffold's shared provider adds `auth.isAuthenticated`. Every
page also receives `errors`.

- `resources/js/Pages/` — one component per Inertia page. The file path without
  its extension is the component string passed to `renderer.Page`.
- `resources/js/Layouts/` — chrome shared by pages. Layouts do not take page
  props.
- `resources/js/Components/` and `resources/js/components/ui/` — reusable
  presentational pieces.
- `resources/js/routes.ts` and `resources/js/types/payloads.ts` — generated; do
  not edit by hand. Use `routes.name.path(...)` for relative URLs and
  `routes.name(...)` for absolute ones, and import prop/form types from
  `@/types/payloads`.
- Import route helpers from `routes.ts`; never invent frontend URLs. Call them
  at render time, not in module-scope constants, because hosts are configured
  after boot.

CSS entry points: `css/base.css` imports Tailwind and `css/theme.css` and holds
`@source` globs; `css/theme.css` is the single owner of the semantic color
tokens. Components use the mapped utilities (`bg-background`,
`text-muted-foreground`, `border-border`, `bg-primary`, `ring-ring`, …);
hand-written CSS uses the raw `var(--…)` properties.

## Queue And Email

Put River argument structs in `queue/jobs/`; put worker implementations,
registration, and periodic jobs in `queue/`. Workers call domain or application
code rather than duplicating domain rules. Registration is a plain slice/map
assembled in `app/`, not Fx.

Recurring work returns a `*river.PeriodicJob` from `queue.PeriodicJobs()`,
which `app.NewQueue` passes to `storage.WithRiverPeriodicJobs`.

Put email templates and send helpers in `email/`. Email is the other templ home
(`email/*.templ` compiled by `emailcompiler`). Put provider-specific adapters in
a top-level package named for the provider (for example `clients/email/`).
`EMAIL_PROVIDER` selects the scaffold transport: `mailpit` (local only, refused
in production), `smtp`, or `log`.

## App Composition

`app/` is the composition root. `app.New` owns the HTTP `Handler`, the
`anduin.Server`, and every dependency between them; `app.NewQueue` owns the
worker graph and processor. `cmd/app` and `cmd/queue` only load environment and
call `lifecycle.Run` around the app's start and stop. `cmd/migrate` and
`cmd/seeds` load environment, open Postgres, run one task, and exit.

Wiring is explicit constructor injection. There is no container.
When adapting an existing application, move the actual constructor graph into
`app/`: construct infrastructure, domains, services and handlers there, then
register handlers on the router before starting the server. Keep entrypoints
small; do not preserve a separate runtime composition package in parallel.

- New config providers: `config/`.
- New domains: construct them in `app/` with `domains.NewX(db)` and pass them
  to the services and handlers that use them. Services that run transactions
  also take the app's `storage.NewTransactor(db)`.
- New services: construct them in `app/` (`services.NewIdentity(...)`).
- New handlers: construct them and call `Register(rt)` in `app/`.
- New queue workers: register them in `app/queue.go`.
- External provider adapters: constructors in their top-level package, wired
  from `app/`.

The dependency graph should be readable top-to-bottom in `app/runtime.go`.

## Decision Checklist

1. Is this a rule about a domain object being valid or allowed? Put it in
   `domains/` (one file per domain).
2. Does it require a transaction, multiple domains, external side effects, or a
   stateful policy check? Put it in `services/`.
3. Is it about HTTP parsing, session/flash, redirects, or rendering? Put it in
   `handlers/`.
4. Is it a page prop or request body? Put it in `payloads/`.
5. Is it only a route path, name, or URL? Put it in `routes/`.
6. Is it app HTTP middleware or a session definition? Put it in `middleware/`.
7. Is it only how a value is displayed? Put it in the relevant Inertia
   component under `resources/js/`.
8. Does it wrap a third-party API with no business rules? Put it in a
   top-level package named for its job.
9. Is it process composition or lifecycle? Put it in `app/`.

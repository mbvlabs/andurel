# Layer Placement

Rules for placing code in an Andurel app. The shape is fixed and deliberately
small:

- HTTP is ordinary Go over `net/http`, reached through
  `github.com/mbvlabs/andurel/pkg/anduin` (see
  [http-and-routing.md](http-and-routing.md)).
- Business logic and persistence live in `domains/`.
- Application logic (cross-domain orchestration) lives in `services/`.
- HTTP adapters live in `handlers/`; route *data* lives in `routes/`; app
  middleware and the cookie jar live in `middleware/`.
- Composition is explicit constructor injection in `internal/runtime/`. There is
  no Fx, no reflection wiring, and no code generator.

Two commands keep an agent honest:

- `andurel patterns show <kind>` prints reference code, the rules for that
  layer, and a "definition of done" checklist. Patterns are read-only; the agent
  writes every file.
- `andurel verify` runs formatting, `go fix`, `go vet`, and the
  domain/service import-boundary lint. Add `--with-tests` to also run
  `go test ./...`.

## Placement Rules

| Code | Put it in |
| --- | --- |
| Database-backed entity structs, `andurel:"column"` table tags, create/update input structs, validation, persistence methods, finder/query methods, domain errors | `domains/` (for example `domains/user.go`) |
| SQL for a domain (narsilc input) | `domains/<entity>.sql`, colocated with the entity |
| narsilc-generated query types and rows (narsilc output) | `domains/internal/queries/` — imported only by `domains/**` |
| Cross-domain vocabulary: `ErrNotFound`, `ErrConflict`, `ErrViolation`, `ValidationError{Fields}` | `domains/errors.go` (package `domains`) |
| Rules that must hold regardless of HTTP, Inertia, API, jobs, or seeds | `domains/` |
| Multi-step workflows, transactions, cross-domain coordination, external side effects, application policy that requires current database state | `services/` |
| Request parsing, path/query/form/JSON binding, HTTP status/render/redirect choices, flash messages, CSRF/session handling | `handlers/` (one flat package) |
| Route names, route paths, URL builders | `routes/` |
| App HTTP middleware (auth, rate limit) and cookie-jar wiring | `middleware/` |
| Inertia root document | `resources/root.templ` (package `resources`) |
| Inertia pages and reusable frontend components | `resources/js/Pages/`, `resources/js/Layouts/`, `resources/js/Components/`, `resources/js/components/ui/` |
| Generated TS route/payload helpers | `resources/js/routes.ts`, `resources/js/types/payloads.ts` (written by `andurel sync routes` / `sync payloads`) |
| Source CSS and theme primitives | `css/` |
| Compiled/static assets | `assets/` |
| SQL schema changes | `migrations/` |
| Apply pending SQL (one-shot process) | `cmd/migrate/` |
| Seed data | `seeds/` or `cmd/seeds/` following the existing pattern |
| External provider adapters | A package under `internal/` named for its job (for example `internal/clients/`, `internal/payment/`) |
| Email templates and send helpers | `email/` |
| River job argument types | `queue/jobs/` |
| River workers and queue registration | `queue/` |
| App configuration structs and environment loading | `config/` |
| Process composition (constructor wiring, infrastructure, process lifecycle) | `internal/runtime/` |
| Process entry (load env, signal context, single `runtime.New` / `runtime.NewQueue`) | `cmd/app/`, `cmd/queue/` |
| One-shot database processes | `cmd/migrate/`, `cmd/seeds/` |

## Domains

`domains/` is all business logic and persistence in one flat package — one
file per domain (`domains/user.go`, `domains/token.go`). A domain answers
"what is true about this entity?" and owns the SQL that reads and writes it.

Use a domain file for:

- Entity structs with newtypes for every invariant (`Email`, not `string`).
  `ParseX` constructors are the supported way to build values; direct
  conversion bypasses validation. There are no post-construction `Validate`
  methods. Keep generated fields mapped to the table (`andurel:"column"`
  tags). When the entity is not the scan row, implement
  `Transform(queries.XRow) error`; keep `sql.Scanner` off the entity.
- Generated `queries.XRow` / `queries.QueryNameRow` values. Domain code returns
  the entity, never the row.
- `Create<Entity>Data` and `Update<Entity>Data` input structs.
- `ParseX` constructors returning `(T, error)` with field normalization
  (trim, lowercase) and `ValidationError` failures.
- CRUD methods, finder methods, pagination, count queries, and domain query
  methods.
- Domain errors. Return the shared sentinels (`ErrNotFound`, `ErrConflict`,
  `ErrViolation`) or a `ValidationError` for field-level failures.

Hard rules:

- **One file per domain; keep cross-domain rules in services.** If a rule spans
  `user` and `token`, it is application logic and belongs in a service.
- **Only `domains` may import `domains/internal/queries`.** `services/`,
  `handlers/`, `internal/runtime/`, `queue/`, and `seeds/` call domain
  constructors and methods; they never touch the generated queries package.
- **SQL is colocated with its domain** (`domains/user.sql`). Narsilc globs
  `domains/**/*.sql` and writes to `domains/internal/queries/`. You never
  place SQL under `models/` — there is no `models/`.
- **Type-driven by default.** The Go domain type is the source of truth; SQL,
  generated queries, and validation follow it. Inspect the mapping with
  `andurel inspect domains --json`. Hand-written SQL is the exception, not the
  default.
- Keep domain methods independent of HTTP, `anduin`, Inertia, sessions, cookies,
  flash messages, and route names.

Good domain responsibilities:

- "A featured project requires a description."
- "A slug must be valid."
- "Create a project entity from domain input and validate it before insert."
- "Find a published item by slug."

Avoid orchestration in a domain when it coordinates multiple entities under one
policy or transaction; that belongs in a service.

## Services

Create or extend a service when the use case is bigger than one domain
operation.

Use services for:

- Transactions spanning more than one domain.
- Cross-domain coordination.
- Workflow policies that require current database state.
- Coordinating persistence with email, queue insertion, cache invalidation, or
  external clients.
- Shared application use cases used by more than one delivery surface.

A service declares its dependencies as constructor arguments and may depend on
several domains, the queue, email, and config:

```go
func NewIdentity(
	users user.Users,
	tokens token.Tokens,
	queue storage.InsertQueue,
	authCfg config.Auth,
	mailCfg config.Mail,
) Identity
```

Do not create a service for a pass-through wrapper around one domain method.
Call the domain directly from the handler until a real workflow appears. There
is no `domains/identity`; `identity`, `authentication`, `registration`, and
`reset_password` live in `services/`.

Good service responsibilities:

- "Create a featured project only if fewer than three projects are featured,
  using a serializable transaction."
- "Register a user, create identity records, issue a token, and enqueue an
  email."
- "Reset a password after validating a token and updating dependent records."

## Handlers

Handlers are thin HTTP adapters. A handler is ordinary Go over a context:

```go
func (h Handler) Index(ctx context.Context) error
```

Use handlers for:

- Reading path values, query strings, headers, and cookies with `anduin`.
- Binding JSON or form payloads with `anduin.DecodeJSON` / `anduin.DecodeForm`.
- Converting HTTP/admin form input into domain data structs.
- Calling a domain or service.
- Mapping errors into pages, validation errors, redirects, flash messages, or
  API responses.
- Rendering Inertia pages through `github.com/mbvlabs/andurel/pkg/inertia`.
- Registering routes in `Register(rt *anduin.Router)`.

Each handler declares its dependencies in a distinct `NewXHandler`
constructor and registers its routes. `handlers/` is one flat package: no
sub-packages, no import aliases.

```go
func NewUserHandler(users user.Users, renderer *inertia.Renderer) UserHandler

func (h UserHandler) Register(rt *anduin.Router) {
	rt.Get(routes.UserIndex.Path(), h.Index)
}
```

Do not put durable business rules in handlers. If the same rule must apply
outside one HTTP action, move it to a domain or service. Error mapping is
uniform: return the error and let the router's `ErrorHandler`
(`handlers.MapError`) translate `Err*` into the right response. See
[http-and-routing.md](http-and-routing.md) for the full request/response and
error API.

Use `AuthHandler` in `handlers/auth.go` for sessions, registration,
confirmation, and password reset, and `APIHandler` in `handlers/api.go` for
JSON endpoints. Parse payloads into domain newtypes at the edge with `ParseX`
before calling services.

## Routes

Route *data* lives in `routes/`. A route is a value built with the
`github.com/mbvlabs/andurel/pkg/routing` constructors and declared as a package
variable so handlers can register it and pages can build URLs:

- `routing.NewSimpleRoute` for fixed paths.
- `routing.NewRouteWithSlug` for slug params.
- `routing.NewRouteWithSerialID` for integer ID params.
- `routing.NewRouteWithUUIDID`, `routing.NewRouteWithToken`,
  `routing.NewRouteWithFile`, `routing.NewRouteWithParams` as needed.

Route values expose `Name() string`, `Path() string`, `URL(params ...string)
string`, `FullURL(...) string`, `Host() routing.HostName`, and
`IsInertia() bool`.

Handlers and services refer to these route values for registration and URL
generation. Do not scatter raw path strings through handlers and pages when a
route value should exist. After adding or changing routes, run
`andurel sync routes --json` so `resources/js/routes.ts` stays in sync.

The `Router` type itself is not app code; it lives in `pkg/anduin`. There is no
`router/` package.

## Middleware

App-specific HTTP policy lives in `middleware/`.
Middleware has the `anduin.Middleware` shape (`func(anduin.Handler)
anduin.Handler`):

- Auth and authorization middleware.
- Rate limiting (`IPRateLimiter`), backed by the injected `cache.Store` so
  every replica enforces the same limit on Redis.
- Route attribute / telemetry helpers.
- CSRF strategy construction.

Cookie jar construction lives in `internal/runtime` (`newJar`); `middleware/`
exports the session definition (`NewAppCookie`) and policy only. There is no
`middleware.NewJar`.

Generic middleware (`Recover`, `Logger`, `Telemetry`, `CORS`, `CSRF`) is
provided by `pkg/anduin` and used directly; do not wrap or reimplement it
here. Session/bag values travel
on `context.Context` through `pkg/kiks` (`kiks.Get[T]`, `kiks.Set`,
`kiks.Destroy`). See [http-and-routing.md](http-and-routing.md).

## Resources And Frontend

Use `resources/root.templ` for the one page-shaped templ in the app: the Inertia
root document (package `resources`). It is the only templ outside `email/`.

Use `resources/js/Pages/` for Inertia screens and `resources/js/Components/` or
`resources/js/components/ui/` for reusable UI. Keep page components focused on
UI state and form behavior; preserve domain validation in Go domains/services.

Keep presentation adapters out of domains when they exist only for display
(label maps, initials and fallback display text, date labels, JSON-to-list
formatting).

Never invent frontend URLs: import `resources/js/routes.ts` after
`andurel sync routes --json`. Import page/form types from
`resources/js/types/payloads.ts` after `andurel sync payloads --json`.

## Queue And Email

Put River argument structs in `queue/jobs/`.

Put worker implementations, queue registration, and queue helpers in `queue/`.
Workers call domain/application code rather than duplicating domain rules.
Registration is a plain slice/map assembled in `internal/runtime`, not Fx.

Put email templates and send helpers in `email/`. Email is the other templ
home (`email/*.templ` compiled by `emailcompiler`). Put provider-specific
implementations in a package under `internal/` (for example
`internal/clients/email/`).

## Internal Packages

Use `internal/` for reusable framework-like support code that is not owned by
one resource, and for adapters to third-party services:

- Process composition (`internal/runtime/`).
- External provider adapters (for example `internal/clients/`,
  `internal/payment/`).
- Request context helpers, storage abstractions, validation helpers, server
  support.

Give each package a name that says what it does. An adapter wraps an external
API and must not contain business rules; services decide when and why to call
it. Do not put app-specific domain workflows in `internal/` just to make them
feel shared. Use `domains/` or `services/`.

`internal/` in a generated app is application-owned. `internal/runtime/`
constructs the whole dependency graph bottom-up with one constructor per
process (`runtime.New` owns `Handler`+`Server`; `runtime.NewQueue` owns the
worker graph) and owns process lifecycle. `cmd/app` and `cmd/queue` only load
environment, create the signal context, and call into `runtime`.
`cmd/migrate` and `cmd/seeds` load environment, open Postgres, run one task,
and exit. Do not apply migrations from `cmd/app` start.

## Dependency Wiring

Wiring is explicit constructor injection in `internal/runtime`. There is no
container.

- New config providers: `config/`.
- New domains: construct them in `internal/runtime` (for example
  `users := user.NewUsers(db)`).
- New services: construct them in `internal/runtime` (`services.NewIdentity(...)`).
- New handlers: construct them and call `Register(rt)` in `internal/runtime`.
- New queue workers: register them in `internal/runtime/queue.go`.
- External provider adapters: constructors in their `internal/` package,
  provided from `internal/runtime`.
- Process entry: `cmd/app/main.go` and `cmd/queue/main.go` for env loading, the
  signal context, and starting `runtime`/`anduin.Server`.
- One-shot database processes: `cmd/migrate/main.go` applies pending SQL;
  `cmd/seeds/main.go` runs named seed sets.

Prefer constructor arguments over package globals. The dependency graph should
be readable top-to-bottom in `internal/runtime/runtime.go`.

## Decision Checklist

Before adding code, answer:

1. Is this a rule about a domain object being valid or allowed? Put it in
   `domains/` (one file per domain).
2. Does it require a transaction, multiple domains, external side effects, or a
   stateful policy check? Put it in `services/`.
3. Is it about HTTP parsing, session/flash, redirects, or rendering? Put it in
   `handlers/`.
4. Is it only a route path, name, or URL? Put it in `routes/`.
5. Is it app HTTP middleware or cookie-jar wiring? Put it in `middleware/`.
6. Is it only for how a value is displayed? Put it in the relevant Inertia
   component under `resources/js/`.
7. Does it only wrap a third-party API with no business rules? Put it in a
   package under `internal/` named for its job.
8. Is it process composition or lifecycle? Put it in `internal/runtime/`.

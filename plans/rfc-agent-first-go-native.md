# RFC: Agent-first, Go-native Andurel

- **Status:** Draft for review — rev 5 (incorporates exploration review)
- **Date:** 2026-10-13
- **Scope:** A **hard roll-forward of v2** on the existing `github.com/mbvlabs/andurel/v2` module path. No compatibility period, no shims. Not a v3 fork.
- **Approach:** **One branch ports everything at once** (§16) so the new shape can be explored locally in one piece.
- **Relates to / supersedes:** `plans/version-two.md`, `plans/standalone-inertia-and-ssr.md`, `plans/andurel-owned-inertia-v3.md`, `plans/remove-di-mode.md`, `plans/testing-rework.html`
- **Review command:** `plannotator annotate plans/rfc-agent-first-go-native.md --tailscale`
- **Changelog rev 5:** flatten `handlers/` to one package (§4, §7); jar construction moves to `internal/runtime`, `middleware/` exports policy only (§4, §8, §10); single-constructor `runtime.New` owns `Handler`+`Server` (§10); domains are type-driven newtypes, parse-don't-validate (§5, §11); `verify` vs `doctor` split clarified, no merge (§12); `patterns show` gains `--entity/--table` builder output (§11, §13).

---

## 1. Summary

Andurel is already the framework that hands you Go you own. This RFC removes every abstraction whose
only job is to be *almost* Go, and re-shapes the generated application so a coding agent can (a) know
exactly where code belongs, (b) write it without a code generator, and (c) prove it works without a
human in the loop.

Three forces drive the design:

1. **Be Go, not a dialect of Go.** Drop `echo.Context`, drop Uber Fx, and drop server-rendered
   component templating for pages. Use `net/http`, `http.ServeMux`, and constructor injection.
2. **Domains are business logic; services are application logic.** Entity rules and persistence live
   in `domains/<entity>` packages that never import each other. Cross-domain orchestration lives in
   `services/`. No DDD ceremony.
3. **Agents, not generators.** `andurel generate …` stops writing application code. It publishes
   **read-only patterns** an agent reads and adapts. `andurel new` keeps producing one complete,
   working reference application (auth) that doubles as the pattern book.

The human stops being the implementer of application code. The framework's job becomes: a place for
everything, a pattern for everything, and a test primitive for every layer.

Because this touches `layout/`, `generator/`, `pkg/`, the CLI, docs, and goldens at once, the port
lands as **one branch** that replaces the old shape wholesale (§16) rather than a phased or
opt-in migration.

---

## 2. Why now (problems with the current shape)

Concrete findings from the current codebase that motivate each change:

| Area | Current reality | Cost |
| --- | --- | --- |
| HTTP | Every handler is `func(etx *echo.Context) error`; `Router.AddRoute` takes `echo.Route`; CORS/CSRF/Recover come from `echo/v5/middleware` (`layout/templates/router_router.tmpl`). | The most-imported type in application code is not Go. Agents must learn Echo. `net/http` knowledge doesn't transfer. |
| DI | `fx.Provide` / `fx.Invoke` / `fx.Annotate` / `fx.As` / `fx.Self` / `fx.Lifecycle`, and generators that edit `controllers/controller.go`, `models/model.go`, `queue/workers.go` to inject into FX modules (`generator/controllers/main_injector.go`, `model_manager.go`). | Wiring is implicit; the generator does source surgery; tests boot a container. "Where does X come from?" is answered by reflection, not reading a file. |
| Views | Two view stacks: templ+Datastar and Inertia, with `.templ`/`_templ.go` pairs, a `renderPage` cache keyed on `templ.Component`, and Datastar SSE/`data-on:` helpers in `pkg/hypermedia`. | Two frontends to maintain, document, generate, and test. Datastar is a whole extra mental model. |
| Domain | `models/` mixes entity, queries, auth crypto, and pagination in one package; cross-model workflows live in a separate `services/` package. | The boundary between "business rule" and "application flow" is fuzzy and unenforced. |
| Generators | `generate model|controller|scaffold|job|email|query` write files and do AST merging into existing source; `sync` regenerates. | The generator is a second implementation of the app. Shape changes require re-teaching it, and it edits user files. |
| Verification | Golden tests exist for the CLI, but there are no first-class per-layer test primitives; scaffolded handler tests still spin up `echo.New()` (`generator/templates/controller_test.tmpl`). | Agents can't cheaply prove a change works at the domain, handler, or integration level. |

---

## 3. Design principles

1. **Native Go over framework idiom.** If the standard library can do it, the standard library does
   it. A convenience must pay for itself twice.
2. **Explicit over injected.** Dependencies are constructor arguments, visible by reading one file.
3. **One owner per concern.** Business rules in `domains/`. Application flows in `services/`. HTTP in
   `handlers/`. Routes in `routes/`. Composition in `internal/runtime`.
4. **Domains are islands.** A domain package never imports another domain package. If two domains
   must cooperate, a service coordinates them.
5. **Patterns are the API.** The CLI teaches patterns; it does not write applications. The generated
   reference app is the executable pattern book.
6. **Every layer is testable in isolation, and together.**
7. **No hidden control flow.** No container, no reflection-based wiring, no request-scoped service
   locator.
8. **The agent is the only reader that matters.** Structure, naming, docs, and CLI output are
   optimized for machine consumption **only**. Human readability is a happy side effect.

---

## 4. The target application shape

### 4.1 Before → after (top level)

| Current | Target | Notes |
| --- | --- | --- |
| `controllers/`, `controllers/api`, `controllers/admin` | `handlers/` (one flat package) | Go name: HTTP handlers. No sub-packages; one `package handlers` with `NewUserHandler`, `NewAuthHandler`, `NewAPIHandler` constructors. A shared-helper file is illustrative, not mandated. |
| `models/` (+ `models/queries`, `models/internal/queries`, `models/factories`) | `domains/` (+ `domains/user`, `domains/token`, `domains/internal/queries`) | Business logic + persistence. SQL is colocated with its domain. Factories are dropped. |
| `services/` | `services/` (unchanged role) | Application logic / cross-domain orchestration. |
| `router/` (Router + middleware + cookies + routes) | `routes/` + `middleware/` | The Router type moves to `pkg/anduin`. Only route *data* stays in the app. Cookie jar *construction* lives in `internal/runtime`; `middleware/` exports only session definitions and policy middleware. |
| `views/` | *(removed)* | `resources/root.templ` keeps the Inertia root document only. |
| `internal/runtime/` | `internal/runtime/` (unchanged home) | Still the composition root; now plain constructors instead of Fx modules. |
| `cmd/app`, `cmd/queue`, `cmd/migrate`, `cmd/seeds` | unchanged, thinner | Each is env load + `runtime.*`. |
| `resources/js/` | unchanged | Inertia pages stay here. The root document joins them under `resources/`. |

### 4.2 Target tree

```
.
├── internal/runtime/         # composition root (was Fx modules) — stays here
│   ├── runtime.go            #   App struct, New(ctx, cfg) (*App, error) — owns Handler + Server + jar
│   └── queue.go              #   worker process wiring (Queue struct, NewQueue)
├── config/                   # app-owned config, one type per subsystem
├── domains/                  # ALL business logic and persistence; domains never import domains
│   ├── errors.go             #   package domains: ErrNotFound, ErrConflict, ErrViolation, ValidationError
│   ├── user/
│   │   ├── user.go           #     newtypes + Users receiver + Parse constructors + persistence
│   │   ├── user.sql          #     SQL for this domain (narsilc input)
│   │   └── user_test.go
│   ├── token/
│   │   ├── token.go
│   │   └── token.sql
│   └── internal/queries/     #   narsilc output (only domains may import)
├── services/                 # application logic / cross-domain orchestration
│   ├── service.go
│   ├── identity.go
│   └── authentication.go
├── handlers/                 # HTTP adapters, ONE package (was controllers)
│   ├── handlers.go           #     shared helpers e.g. MapError (illustrative)
│   ├── pages.go              #     NewPagesHandler + Register
│   ├── user.go               #     NewUserHandler(users, renderer) + Register
│   ├── auth.go               #     NewAuthHandler(identity, renderer) + Register
│   ├── api.go                #     NewAPIHandler(db) + Register
│   └── assets.go             #     NewAssetsHandler + Register
├── routes/                   # route definitions only (no Router type)
├── middleware/               # app HTTP policy only: auth guard, rate limit, session Definition
├── resources/
│   ├── root.templ            # Inertia root document (templ: the one page-shaped templ left)
│   ├── root_templ.go         # generated
│   └── js/                   # Inertia app, Pages/, Layouts/, types/
├── email/                    # templ + emailcompiler (unchanged; the other templ home)
├── queue/                    # jobs/ + workers (no FX; explicit worker registry)
├── migrations/
├── seeds/
├── assets/                   # compiled/static assets
├── css/
└── internal/                 # third-party provider adapters only (clients/, payment/, …)
```

### 4.3 No sub-packages under handlers (rev 5)

`handlers/` is one flat `package handlers`. Sub-packages (`handlers/user`,
`handlers/auth`, `handlers/api`) buy a second import block and force the
`domainuser` alias dance wherever a domain and its handler meet, for no
isolation benefit — handlers share the renderer, the error mapper, and the
router type.

Distinct constructors disambiguate instead: `NewUserHandler`,
`NewAuthHandler`, `NewAPIHandler`, `NewPagesHandler`, each with a `Register(rt
*anduin.Router)` method. Generic caches, builders, or infra helpers do not live
here; they belong in `internal/runtime` or the owning package.

---

## 5. Domains

### 5.1 Shared domain vocabulary: `domains/errors.go`

The root `domains` package holds only cross-domain vocabulary. It has no subpackage dependencies (no
import cycles).

```go
// domains/errors.go
package domains

import "errors"

var (
	ErrNotFound  = errors.New("domain: not found")
	ErrConflict  = errors.New("domain: conflict")
	ErrViolation = errors.New("domain: violation")
)

// ValidationError carries field-level messages that handlers map to 422 / Inertia props.errors.
type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string     { return "domain: validation failed" }
func (e ValidationError) Is(target error) bool { return target == ErrViolation }
```

### 5.2 Entity domain: `domains/user/user.go` (rev 5: type-driven, parse-don't-validate)

Domains follow Palmieri-style type-driven development ("Using Types To Guarantee
Domain Invariants", Zero to Production ch. 6): invariants live in newtypes with
private state, the `ParseX` constructor is the *only* way to build one outside
the domain package, and read access is read-only. There is no exported
`Validate() *User` called after construction — an invalid value is
unrepresentable, so `insert`-style callees never re-check.

```go
package user

// andurel:table users

import (
	"context"
	"errors"
	"strings"
	"uuid"

	"app/domains"
	"app/domains/internal/queries"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/jackc/pgx/v5"
)

type Users struct{ q *queries.Queries }

func NewUsers(db storage.Connection) Users            { return Users{q: queries.New(db)} }
func (u Users) WithTx(tx storage.Transaction) Users   { return Users{q: queries.New(tx)} }

// Email is a parsed, normalised address. ParseEmail is the sole constructor.
type Email string

func ParseEmail(raw string) (Email, error) {
	v := Email(strings.ToLower(strings.TrimSpace(raw)))
	if err := validation.ValidateEmail(string(v)); err != nil {
		return "", domains.ValidationError{Fields: map[string]string{"email": err.Error()}}
	}
	return v, nil
}

func (e Email) String() string { return string(e) }

type User struct {
	ID    uuid.UUID `andurel:"id"`
	Email Email     `andurel:"email"`
}

type CreateUserData struct {
	Email Email // already parsed; handlers/services parse at the edge
}

func (u Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
	entity, err := u.q.GetUser[User](ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, domains.ErrNotFound
	}
	return entity, err
}

// Create / Update / Destroy / All / Paginate as today, returning domain errors.
// Create takes CreateUserData (parsed types), never raw strings.
```

Rules: one newtype per invariant (`Email`, `PasswordHash`, `TokenScope`, …);
fields stay unexported or expose value-only accessors (`String()`, `Bytes()`);
`Parse` returns `(T, error)` mapping failures to `domains.ValidationError`;
persistence methods accept parsed inputs; unit tests target `Parse` directly.

### 5.3 SQL colocated with its domain

Narsilc stops scanning a single `models/queries/` directory. Queries live beside the domain that
owns them:

```
domains/user/user.sql      # hand-written
domains/token/token.sql
domains/internal/queries/  # narsilc output (all generated types)
```

The narsilc configuration in `andurel.lock` changes to glob `domains/**/*.sql` with the output
package fixed at `domains/internal/queries`. `pkg/storage/narsilc.go` (`QueriesDir`,
`GeneratedQueriesDir`, `HasQueryFiles`) is updated to match.

### 5.4 What "DB logic only under domains" means in practice

- Only `domains/**` imports `domains/internal/queries` and issues SQL.
- `services/`, `handlers/`, `internal/runtime/`, `queue/`, and `seeds/` call domain constructors and
  methods; they never import the generated queries package.
- `internal/` adapters wrap third-party APIs and hold no business rules.
- Transactions use `storage.RunInTransaction` inside a domain method or a service.
- **A domain package must not import another domain package.** A rule that spans `user` and `token`
  is application logic and belongs in a service.

This is enforceable mechanically (see §12): an `andurel verify` import check fails when a file
outside `domains/` imports `domains/internal/queries`, or when one domain imports another.

---

## 6. Services (application logic)

`services/` keeps its role — it is where use cases live — but its boundary is now sharp:

- A **domain** answers "what is true about this entity?" and owns its own persistence.
- A **service** answers "what should happen across entities, surfaces, or side effects?" It may
  depend on several domains, the queue, email, and config. It is not a pass-through wrapper.

```go
// services/identity.go
package services

type Identity struct {
	users  user.Users
	tokens token.Tokens
	queue  storage.InsertQueue
	pepper string
}

func NewIdentity(
	users user.Users,
	tokens token.Tokens,
	queue storage.InsertQueue,
	authCfg config.Auth,
	mailCfg config.Mail,
) Identity { /* ... */ }

func (i Identity) AuthenticateUser(ctx context.Context, data LoginData) (user.User, error) {
	// coordinates user + token + email; the only place these domains meet
}
```

`identity`, `authentication`, `registration`, and `reset_password` stay in `services/`. There is no
`domains/identity`.

---

## 7. Handlers

### 7.1 Handler shape

`controllers/` → `handlers/`. Handlers are thin HTTP adapters. The goal is that a handler body is
regular Go over `context.Context`:

```go
func (h Handler) Index(ctx context.Context) error
```

`pkg/anduin` (see §8) supplies request/response access through that context. No `echo.Context`, no
custom context struct, no per-request service locator — only transport request metadata on the
context.

### 7.2 Constructor injection: `NewXHandler` (rev 5: flat package)

Each handler declares its dependencies in a constructor. One package, distinct
names — no aliases, no sub-package imports.

```go
// handlers/user.go  (illustrative)
package handlers

import (
	"context"

	"app/domains/user"
	"app/routes"
	"app/middleware"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

type UserHandler struct {
	users    user.Users
	renderer *inertia.Renderer
}

func NewUserHandler(users user.Users, renderer *inertia.Renderer) UserHandler {
	return UserHandler{users: users, renderer: renderer}
}

func (h UserHandler) Register(rt *anduin.Router) {
	rt.Get(routes.UserIndex.Path(), h.Index)
	rt.Get(routes.UserShow.Path(), h.Show)
	rt.Post(routes.UserCreate.Path(), h.Create, middleware.RateLimit(5))
}

func (h UserHandler) Index(ctx context.Context) error {
	page, err := anduin.QueryInt(ctx, "page", 1)
	if err != nil {
		return anduin.BadRequest(err)
	}

	result, err := h.users.Paginate(ctx, page, 25)
	if err != nil {
		return err
	}

	return h.renderer.Page(ctx, "User/Index", inertia.FromStruct(result.Items)).Render()
}
```

### 7.3 Shared handler helpers (illustrative)

Today response composition lives in `pkg/hypermedia`/Inertia. App-specific error mapping and response
composition can live in a shared file in `handlers/`; this is an example, not a required filename or
structure:

```go
// handlers/errors.go  (illustrative)
package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/inertia"
	"github.com/mbvlabs/andurel/pkg/anduin"
)

func MapError(renderer *inertia.Renderer, ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domains.ErrNotFound):
		return renderer.Page(ctx, "Errors/NotFound", nil).Status(http.StatusNotFound).Render()
	case errors.Is(err, domains.ErrViolation):
		if ve, ok := errors.AsType[domains.ValidationError](err); ok {
			return renderer.Page(ctx, "Errors/Validation", nil).
				ValidationErrors(ve.Fields).
				Status(http.StatusUnprocessableEntity).
				Render()
		}
	}
	return anduin.Error(ctx, http.StatusInternalServerError, err)
}
```

The router's error handler is configured in `internal/runtime` to call this, so handlers just
`return err` and mapping is uniform — fixing today's "controllers leak internal errors"
(C6/S6 in `docs/v2-release-readiness-review.md`).

### 7.4 Auth handlers

`handlers/auth.go` (`AuthHandler`) hosts sessions, registration, confirmation, and
password reset, mirroring today's `controllers_sessions_inertia.tmpl` etc. These are the reference
patterns shipped by `andurel new`.

---

## 8. HTTP: `pkg/anduin` — one package for all HTTP primitives

### 8.1 What changes and why

- **`pkg/anduin` absorbs `pkg/server`.** The two modules merge into a single HTTP package so every
  HTTP primitive an app needs — routing, the handler model, middleware, request/response helpers, the
  error model, and the listener/lifecycle from today's `pkg/server` — is exported from one place.
  `pkg/server` is deleted; its `Server` (address, timeouts, `Shutdowners`, graceful shutdown) moves
  into `anduin` unchanged in spirit.
- **`pkg/routing` stays** as the pure route/URL/host model (it is already echo-free). It is not
  merged into anything; `anduin` consumes it.
- **No new `pkg/web`.** `pkg/anduin` (a nod to the great river) is a thin layer over `net/http` and
  `http.ServeMux` (Go 1.22+ method/wildcard patterns) — not a from-scratch path matcher.
- **The routing primitive is `Router`, not `Server`.** Registration and dispatch (`router.Get(…)`,
  `router.Handler()`) are a different responsibility from owning a listener. Keeping them as two
  types in one package means a test and a production process use the *same* `Router`, while only the
  process pays for a `Server`. `Router.Handler()` returns an `http.Handler` that `Server` consumes,
  so the router stays independently testable and embeddable.
- The application's `router/` package is deleted. Generic middleware (Recover, Logger, Telemetry,
  CORS, CSRF) moves to `pkg/anduin` and is used directly — no thin re-export wrappers in the app;
  app-specific policy (auth, rate limit) plus the session `Definition` live in
  `middleware/`. Cookie jar *construction* (`kiks.NewJar`) lives in `internal/runtime` (see §10),
  not in `middleware` — there is no `middleware.NewJar`.

### 8.2 Handler and router API

```go
// pkg/anduin/handler.go
package anduin

type Handler func(ctx context.Context) error
type Middleware func(Handler) Handler
type ErrorHandler func(ctx context.Context, err error) error

// pkg/anduin/router.go
type Router struct {
	primary *http.ServeMux
	hosts   map[routing.HostName]*http.ServeMux
	mw      []Middleware
	onError ErrorHandler
}

func NewRouter(opts ...Option) *Router

func (rt *Router) Get(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Post(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Put(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Patch(pattern string, h Handler, mw ...Middleware)
func (rt *Router) Delete(pattern string, h Handler, mw ...Middleware)

func (rt *Router) Host(name routing.HostName) *Host // named virtual host
func (rt *Router) Use(mw ...Middleware)
func (rt *Router) Handler() http.Handler // host dispatch + middleware + error adapter

// pkg/anduin/server.go — folded in from pkg/server
type Server struct { /* *http.Server + Shutdowners */ }
type ServerOptions struct { IdleTimeout, ReadTimeout, WriteTimeout time.Duration }
type ServerOption func(*ServerOptions)
type Shutdowner interface { Shutdown(ctx context.Context) error }

func NewServer(ctx context.Context, host, port, env string, handler http.Handler,
	shutdowners []Shutdowner, opts ...ServerOption) Server
func (s *Server) Start(ctx context.Context) error
```

`Handler()` returns an `http.Handler` for the package's `Server` (or any `net/http` consumer).
Internally it dispatches on `r.Host` to the right `http.ServeMux`, runs the middleware chain, stores
the request/writer on the context, calls the `anduin.Handler`, and routes any returned error to
`onError`.

### 8.3 Request/response access from `ctx`

```go
// request
func Request(ctx context.Context) *http.Request
func ResponseWriter(ctx context.Context) http.ResponseWriter
func PathValue(ctx context.Context, key string) string
func Query(ctx context.Context, key string) string
func QueryInt(ctx context.Context, key string, def int) (int, error)
func DecodeJSON(ctx context.Context, dst any) error
func DecodeForm(ctx context.Context, dst any) error

// response
func JSON(ctx context.Context, status int, v any) error
func Text(ctx context.Context, status int, s string) error
func HTML(ctx context.Context, status int, s string) error
func NoContent(ctx context.Context, status int) error
func Redirect(ctx context.Context, url string, status int) error

// errors
type HTTPError struct { Status int; Message string; Err error }
func (e *HTTPError) Error() string
func (e *HTTPError) Unwrap() error
func Error(ctx context.Context, status int, err error) error
func NotFound(err error) error
func BadRequest(err error) error
```

Only transport data (request, writer, path params) travels on the context. Dependencies and database
handles never do — consistent with `plans/version-two.md`.

### 8.4 Middleware

```go
func Recover() Middleware
func Logger(log *slog.Logger) Middleware
func Telemetry(tel *telemetry.Telemetry) Middleware
func CORS(cfg CORSConfig) Middleware
func CSRF(cfg CSRFConfig) Middleware
```

`pkg/kiks` drops `EchoMiddleware`/`echo.UnwrapResponse` and gains a native middleware. Its
`persistWriter` is already an `http.ResponseWriter` wrapper.

### 8.5 Route path metadata

Middleware that needs the matched route (telemetry `http.route`, logging) uses `mux.Handler(r)` to
obtain the pattern, replacing `c.RouteInfo()`.

---

## 9. Views and frontend

### 9.1 templ/Datastar are removed as page options

- `--ui templ/datastar` is removed. The only UI options are Inertia (`react|vue|svelte`).
- `pkg/hypermedia` is deleted (it exists only to render templ fragments and Datastar SSE). The
  vendored `datastar_1-0-1.min.js` asset is deleted.
- The view generator, `resource_view*.tmpl`, `views/*`, and the `pageCache[templ.Component]` in
  `controllers_controller.tmpl` are deleted.
- `sync views` is removed.

### 9.2 templ stays in exactly two places

1. **Email** — `email/*.templ`, `emailcompiler/`, and `generator/templates/email.tmpl` are unchanged.
2. **The Inertia root document** — moved from `views/root.templ` to `resources/root.templ`
   (package `resources`).

```go
// resources/root.templ
package resources

import "github.com/mbvlabs/andurel/pkg/inertia"

templ Root(data inertia.RootData) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<title>{ data.ProjectName }</title>
			@inertia.SSRHead(data.SSR)
			@templ.Raw(string(data.ViteHead))
		</head>
		<body>
			if data.SSR != nil {
				@inertia.SSRBody(data.SSR)
			} else {
				@inertia.PageScript(data.ContainerID, data.PageJSON)
				@inertia.AppMount(data.ContainerID)
			}
			@templ.Raw(string(data.ViteBody))
		</body>
	</html>
}
```

`pkg/inertia` keeps `RootFunc`/`PageScript`/`AppMount`/`SSRHead`/`SSRBody` (templ) but loses echo and
takes `context.Context`, obtaining request/response through `anduin`:

```go
func (renderer *Renderer) Page(ctx context.Context, component string, props Props) *PageBuilder
func (renderer *Renderer) Redirect(ctx context.Context, location string, status int) error
func (renderer *Renderer) Middleware() func(http.Handler) http.Handler
```

### 9.3 Errors and fallbacks

Error pages are Inertia pages (`resources/js/Pages/Errors/*`). The router's `ErrorHandler` maps
`domains.ErrNotFound` etc. to those components. The plain-HTML templ error pages are gone.

---

## 10. Composition: `internal/runtime`, no FX (rev 5: single constructor)

`internal/runtime` stays the composition root and owns *everything* for its
process. It no longer builds Fx modules; it constructs the graph explicitly,
bottom-up. Nothing is discovered by reflection. There is exactly one
constructor per process — `New` for HTTP, `NewQueue` for workers — and no
separate `NewServer` for callers to glue: the `anduin.Server` is built inside
`New` from the same `Handler` the router produced.

`middleware/` exports policy only. The cookie jar is built here, in `newJar`,
from `middleware.NewAppCookie` + `kiks.NewJar` — `middleware.NewJar` does not
exist.

```go
// internal/runtime/runtime.go (illustrative)
package runtime

type App struct {
	Config  config.Config
	DB      storage.Connection
	Handler http.Handler
	Server  anduin.Server
	closers []func(context.Context) error
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	tel, err := telemetry.New(ctx, cfg.App.ProjectName, cfg.Version, cfg.Telemetry.Options()...)
	if err != nil {
		return nil, err
	}

	db, err := storage.NewPostgres(ctx, cfg.Database, storage.WithOpenTelemetry(tel))
	if err != nil {
		return nil, err
	}

	// domains
	users := user.NewUsers(db)
	tokens := token.NewTokens(db)
	queueInsert, err := storage.NewQueueInsert(db, cfg.Queue)
	if err != nil {
		return nil, err
	}

	// services (application logic)
	identity := services.NewIdentity(users, tokens, queueInsert, cfg.Auth, cfg.Mail)

	// presentation
	renderer, err := inertia.New(rendererOptions(cfg), inertia.WithRoot(resources.Root))
	if err != nil {
		return nil, err
	}
	jar, err := newJar(cfg.App, cfg.Session) // kiks.NewJar + middleware.NewAppCookie
	if err != nil {
		return nil, err
	}

	// HTTP
	rt := anduin.NewRouter(
		anduin.WithErrorHandler(handlers.MapError(renderer)),
		anduin.WithTelemetry(tel),
	)
	rt.Use(
		anduin.Recover(),
		anduin.Telemetry(tel),
		jar.Middleware(),
		renderer.Middleware(),
		anduin.CORS(corsConfig(cfg)),
		anduin.CSRF(csrfConfig(cfg)),
	)

	handlers.NewUserHandler(users, renderer).Register(rt)
	handlers.NewAuthHandler(identity, renderer).Register(rt)
	handlers.NewAPIHandler(db).Register(rt)
	handlers.NewAssetsHandler().Register(rt)

	handler := rt.Handler()
	srv := anduin.NewServer(ctx, cfg.HTTP.Host, cfg.HTTP.Port, cfg.App.Environment,
		handler, nil, anduin.WithTimeouts(cfg.HTTP.IdleTimeout, cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout))

	app := &App{
		Config:  cfg,
		DB:      db,
		Handler: handler,
		Server:  srv,
		closers: []func(context.Context) error{db.Close, tel.Shutdown},
	}
	srv.AddShutdowner(app) // App.Close runs on graceful shutdown

	return app, nil
}

func (a *App) Start(ctx context.Context) error       { return a.Server.Start(ctx) }
func (a *App) Shutdown(ctx context.Context) error    { return a.Server.Shutdown(ctx) }
func (a *App) Close(ctx context.Context) error       { /* run closers in reverse */ }
```

`cmd/app/main.go` becomes env load + `runtime.New` + `app.Start` +
`app.Shutdown`. `cmd/queue/main.go` builds the River processor explicitly (`storage.NewQueueProcessor`) and
registers workers from a plain slice/map. `cmd/migrate` and `cmd/seeds` already use `pkg/storage`
directly. Lifecycle start/shutdown ordering is explicit code, not `fx.Hook`. `pkg/anduin` still
keeps `Router` and `Server` as separate types (tests use `Router` alone); only the *app* stops
exposing two constructors.

Why this helps agents: the dependency graph is readable top-to-bottom in one file; tests construct
exactly what they need; adding a handler is one constructor call plus a `Register`.

---

## 11. Generators → patterns

### 11.1 The pivot

`andurel generate model|controller|scaffold|job|email|query` **stops injecting application code**.
It becomes a strictly read-only pattern library that still knows the *complete* shape of every
primitive, exactly like today's generators do:

```
andurel patterns                       # list pattern ids
andurel patterns list --json
andurel patterns show domain           # print reference code + rules + checklist
andurel patterns show domain --json    # machine-readable: files, headers, snippets, checklist
andurel patterns show domain --entity User --table users
                                       # builder output for one entity: ordered file manifest
                                       # (migration + colocated .sql + entity/receiver + test)
```

Each pattern is embedded Markdown with compilable Go/TS snippets, the placement rules from
`skills/andurel/references/layer-placement.md`, and a "definition of done" checklist an agent can
self-verify against. Patterns never write files; the agent writes every file. The old
`generate model|controller|scaffold` convention survives only here: `--entity/--table` is the
replacement for `generate model User`, emitting the same deterministic shape without touching disk.

**Patterns expose primitives, not just prose.** Adding a new domain must be deterministic: the
`domain` pattern emits the *whole* database picture for that entity — table + columns + types +
nullability + constraints + indexes, the colocated `.sql`, the Go newtypes/receiver with `Parse`
constructors, domain errors,
and the migration/query wiring. The generator's deterministic **builder primitives are kept, not
deleted**: the pattern engine calls the same builders that map a domain type to files, but renders
them to stdout (Markdown + `--json`) instead of writing them, so output stays diffable and
golden-testable. Its output is an ordered manifest of files to create, so an agent applying it lands
on exactly the shape a human would. Every `andurel new` scaffold is designed domain-around-table(s)
(e.g. `User` → `users`), so `--entity User` reproduces that convention on demand. Patterns also render
**worked examples** (a real `user` domain,
a real `handler`, a real migration) so each primitive is shown, not merely described.

**Type-driven by default.** The default mode of work is type-driven: the Go domain newtypes (their
`Parse` constructors and tags) are the single source of truth from which SQL, generated queries, and
validation are derived and checked. Patterns document this as the intended flow, and the CLI exposes it
(`andurel inspect domain --json` reports the type→column mapping), so an agent changes the type first
and lets the deterministic primitives follow. Hand-written SQL remains possible, but it is the
exception, not the default.

Pattern catalog (initial):

| Pattern | Covers |
| --- | --- |
| `domain` | complete DB picture: table + columns/types/constraints/indexes + colocated `.sql` + newtypes/`Parse` + entity/receiver + domain errors (`--entity/--table` builder output) |
| `service` | cross-domain orchestration + transaction |
| `handler` | resource handler in flat `package handlers`, payload parsing at edge, error mapping, `Register` |
| `route` | route definition + URL builder + TS route helper |
| `job` | River args + worker + registration |
| `email` | templ email + `emailcompiler` + send helper |
| `migration` | goose migration authoring |
| `test-domain` / `test-handler` / `test-integration` | the per-layer recipes from §12 |
| `composition` | how to wire a new dependency in `internal/runtime` (single `New`, `newJar` included) |

`andurel new` still scaffolds a complete, working app (auth included). That app *is* the executable
pattern book: every pattern has a corresponding real file in the scaffold.

### 11.2 What happens to `sync`

| Command | Fate |
| --- | --- |
| `sync queries` (narsilc, now over `domains/**/*.sql`) | Keep. |
| `sync email` (emailcompiler) | Keep. |
| `sync routes` / `sync payloads` (TS) | Keep. |
| `sync views` | Delete (no templ views). |
| `sync model` | Delete (no generated models). |
| `sync factory` / `sync factories` | Delete (factories dropped). |

---

## 12. Testing and agent self-verification

Factories are **dropped**. Tests build entities inline using the domain API; small in-package test
helpers are fine when they earn their keep.

### 12.1 Per-layer primitives

| Layer | Primitive | What it does |
| --- | --- | --- |
| Domain | `pkg/storage.TestCluster` + `NewTestDB(t, migrations.Migrations, ".")` | Real Postgres, migrations applied, one clean DB per test. |
| Handler | `pkg/anduin/anduintest` | Builds an `httptest` server from real handler registrations; `Do(req)` returns a decoded response + cookie jar. |
| Integration | `runtime.New(ctx, testConfig)` | Boots the entire real graph (no FX) and exposes `Handler` + `DB`. |
| CLI/artifacts | `internal/goldentest` | Existing CLI-driven byte-stable goldens (per `plans/testing-rework.html`). |
| Cross-cutting | `andurel verify` | `gofmt`, `go fix`, `go vet` (shared helper with `doctor`), tests (`--with-tests`), and the domain-boundary import lint. The write-path gate; fails CI. |
| Diagnostics | `andurel doctor` | env, lock, tools, tidy, and generated-file drift (templ/narsilc/routes/payloads). Read-only health; warns, never fails the agent loop. No merge with `verify` (rev 5 decision). |

### 12.2 Illustrative handler test (rev 5: flat handlers, parsed inputs)

```go
func TestUserIndex(t *testing.T) {
	db := testCluster.NewTestDB(t, migrations.Migrations, ".")
	users := user.NewUsers(db)
	for i := range 3 {
		email, err := user.ParseEmail(fmt.Sprintf("u%d@x.y", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := users.Create(t.Context(), user.CreateUserData{Email: email}); err != nil {
			t.Fatal(err)
		}
	}

	srv := anduintest.New(t, func(rt *anduin.Router) {
		handlers.NewUserHandler(users, testRenderer(t)).Register(rt)
	})

	resp := srv.Get(t, routes.UserIndex.Path()+"?page=1")
	resp.AssertStatus(t, http.StatusOK)
	resp.AssertInertia(t, "User/Index")
}
```

### 12.3 Illustrative domain test (rev 5: Parse is the unit)

```go
func TestParseEmail(t *testing.T) {
	if _, err := user.ParseEmail("not-an-email"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUsers_Create(t *testing.T) {
	db := testCluster.NewTestDB(t, migrations.Migrations, ".")
	users := user.NewUsers(db)

	email, err := user.ParseEmail("a@b.c")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := users.Create(t.Context(), user.CreateUserData{Email: email})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID == uuid.Nil {
		t.Fatal("expected id")
	}
}
```

### 12.4 Self-verification loop for agents

1. `andurel patterns show <kind> [--entity X --table y]` — read the pattern and its checklist.
2. Write the code (domain newtypes first → service if needed → handler → route → composition).
3. `andurel verify` — format, vet, domain/service-boundary lint (the gate).
4. `go test ./...` (or `verify --with-tests`) — domain tests need a Postgres container; handler tests need nothing external.
5. `andurel inspect routes --json` / `andurel doctor --json` — confirm the declared shape and drift (diagnostics, not the gate).

### 12.5 Facts the agent can assert against

- Route manifest (`routes/` is data) — no server needed.
- Domain/service package boundaries — lintable from imports (domains never import domains; only
  domains import generated queries).
- Typed errors (`domains.Err*`) — handlers, services, and tests share one vocabulary.

---

## 13. CLI, docs, and agent surfaces

- `inspect controllers` → `inspect handlers` (flat package); `inspect views` removed; add `inspect domains` (reports type→column mapping for the type-driven flow).
- `inspect project --json` reports the new shape (UI adapter is always Inertia now).
- `patterns show domain --entity/--table` documented alongside `generate migration` (the only writer left).
- `cli/command_meta_catalog.go` and `commands --json` updated; `commands --check` keeps docs in sync.
- `skills/andurel/references/layer-placement.md` rewritten for domains/services/handlers/routes/
  `internal/runtime`.
- New embedded reference `skills/andurel/references/http-and-routing.md` describing `pkg/anduin`.
- README and `layout/templates/agents.tmpl` updated: no Echo, no Fx, no templ/Datastar views.

---

## 14. `pkg/` module changes

| Module | Change |
| --- | --- |
| `pkg/routing` | **Kept** unchanged (already echo-free). Not merged anywhere; consumed by `pkg/anduin`. |
| `pkg/server` | **Merged into `pkg/anduin` and deleted.** Its `Server` (address, timeouts, `Shutdowners`, graceful shutdown) moves in. |
| `pkg/anduin` | **The single HTTP package.** Native `Router` over `http.ServeMux`, `func(ctx) error` handlers, request/response helpers, error model, generic middleware, and the merged `Server` lifecycle. |
| `pkg/hypermedia` | **Delete.** |
| `pkg/inertia` | Remove echo; accept `context.Context` and obtain request/response via `anduin`; keep templ root helpers. |
| `pkg/kiks` | Remove `EchoMiddleware`/`echo.UnwrapResponse`; add a native middleware. |
| `pkg/storage` | Narsilc paths → `domains/**/*.sql` and `domains/internal/queries`; consider moving testcontainers behind a build tag / subpackage (fixes C5 in the readiness review). |
| `pkg/telemetry` | `From` keeps the structural `HTTPContext` interface or accepts `*http.Request`; `WrapHandler` unchanged. |
| `pkg/validation`, `pkg/email` | Unchanged. |

Root `go.mod` drops `github.com/labstack/echo/v5` and `go.uber.org/fx`. `pkg/hypermedia` leaves
`go.work`. templ stays (email + Inertia root).

---

## 15. Roll-forward (v2), hard cut

- The module path stays `github.com/mbvlabs/andurel/v2`; this is a v2 roll-forward, not a v3 fork.
- **No compatibility period and no shims.** Echo, Fx, templ views, and Datastar are removed
  outright. The old `layout/`, `controllers/`, `models/`, `views/`, `router/`, and their generators
  are **deleted**, not kept alongside the new shape. `andurel new` produces exactly one shape.
- Existing v2 apps are **not** auto-rewritten by `andurel upgrade`, but the scaffold they upgrade to
  is the new one, and the old Echo scaffolding no longer exists. Migrating an app means moving to
  `domains/`/`services/`/`handlers/`/`pkg/anduin` by hand, guided by the patterns.
- The break is per-package; version bumps are a release-management detail, not a runtime seam.

---

## 16. Migration: one branch, everything at once

This is **not** phased and has **no intermediate compatibility state**. The entire port lands in a
single branch so the new shape can be explored locally as one piece:

- **`pkg/anduin`** — merge `pkg/server` in; native `Router`, `func(ctx) error` handlers,
  request/response helpers, error model, middleware, and the `Server` lifecycle. Convert `pkg/kiks`
  and `pkg/inertia` off echo. Delete `pkg/hypermedia`; drop echo/fx from `go.mod`/`go.work`.
- **`layout/`** — rewrite to `domains/` (+ colocated `*.sql`), `services/`, `handlers/`, `routes/`,
  `middleware/`, `internal/runtime`, `resources/root.templ`, Inertia-only. Delete `views/`, templ
  pages, the Datastar asset, factories, and `models/`.
- **`generator/`** — replace code-writing with `andurel patterns`; delete the AST injectors
  (`main_injector.go`, `planModelRegistration`, worker injection). Keep migrations and
  `sync queries`/`email`/`routes`/`payloads`.
- **Verification** — add `pkg/anduin/anduintest`, `andurel verify`, the domain/service-boundary lint,
  and the three test patterns.
- **Docs** — rewrite skill/README/AGENTS and `cli/command_meta_catalog.go`; regenerate goldens.

`master` may be red mid-branch — the branch *is* the exploration. It closes green: golden updates
plus `just check-fmt && just check-fix && just vet`.

---

## 17. Risks

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Large blast radius across `layout/`, `generator/`, `pkg/`, docs, goldens. | High | One YOLO branch (§16); commit a green checkpoint before each layer and after each golden refresh. |
| Big-bang branch is long-lived and hard to review. | High | It is explicitly an exploration branch; patterns + regenerated goldens make the shape mechanical, and the final diff is mostly deletions. |
| Dropping Fx removes lifecycle/DI ergonomics. | Med | Explicit `runtime.New` + a small ordered `closers` list; test bootstrapping gets simpler. |
| `http.ServeMux` host dispatch + middleware ordering subtleties. | Med | Anduin wraps it with focused tests; virtual hosts are documented and tested. |
| `domains/`/`services/` boundary drift. | Med | `andurel verify` import lint; code review. |
| Removing generators slows humans. | Low | Patterns expose the complete primitive + worked examples; the target user is the agent. |
| templ/Datastar removal alienates existing users. | High | Hard cut with no shim (§15); migration is manual and pattern-guided. |

---

## 18. Decisions locked from review

| # | Decision |
| --- | --- |
| Scope | **Hard v2 roll-forward**: no compatibility period, no shims. One branch ports everything (§16). |
| HTTP layer | **`pkg/anduin`** — the single HTTP package (merged with `pkg/server`), exporting all HTTP primitives. No `pkg/web`. App uses it directly; no middleware re-export wrappers. |
| HTTP primitives | Routing primitive is **`Router`**; the listener/lifecycle **`Server`** lives in the same package. |
| Handler ergonomics | A handler is **`func(ctx context.Context) error`**; request/response access through `anduin` on the context. |
| Handlers layout | **Flat `package handlers`** — no `handlers/user|auth|api` sub-packages; `NewUserHandler`/`NewAuthHandler`/`NewAPIHandler` + `Register`. |
| Routing | **Keep `pkg/routing`**; do not merge it. |
| Composition | **Keep `internal/runtime`, single constructor**: `New` owns `Handler`+`Server`+jar; no `NewServer`, no `middleware.NewJar`. Explicit constructors replace Fx. |
| Domain vs service | Business logic → **`domains/`**; application logic → **`services/`**. **Domains never import each other.** No DDD. |
| Domain modelling | **Type-driven newtypes, parse-don't-validate** (Palmieri ch. 6): private state, `ParseX` sole constructor, read-only accessors, `CreateData` takes parsed types. |
| SQL | Colocated per domain (`domains/user/user.sql`); narsilc globs `domains/**/*.sql`, output `domains/internal/queries`. |
| Root document | **`resources/root.templ`** (package `resources`). |
| Factories | **Dropped**; tests build entities inline. |
| Generators | **Strictly read-only patterns** (`patterns show [--entity/--table]`) that still expose complete deterministic primitives + worked examples. `generate` keeps only `migration`. |
| Development mode | **Type-driven by default**: the Go domain newtypes are the source of truth; SQL/queries follow. |
| Enforcement | **Import lint in `andurel verify` + CI** for the domain/service boundary. `verify` (gate) and `doctor` (diagnostics) stay split; shared `vet` helper, no merge. |
| Optimisation target | **Machine consumption only.** |

---

## Appendix A — Full reference feature (users), target shape

```
domains/user/user.go                 # newtypes + Parse + Users receiver + rules
domains/user/user.sql                # colocated SQL
domains/internal/queries/user.sql.go # narsilc output
services/                            # application logic if the use case spans domains
handlers/user.go                     # package handlers: NewUserHandler(user.Users, *inertia.Renderer) + Register
routes/users.go                      # route data
internal/runtime/runtime.go          # single New wires it all (Handler + Server + jar)
resources/js/Pages/User/Index.vue    # Inertia page
```

## Appendix B — Before/after cheat sheet

| Concept | Before | After |
| --- | --- | --- |
| Handler signature | `func(etx *echo.Context) error` | `func(ctx context.Context) error` |
| HTTP framework | Echo v5 (+ echomw) | `pkg/anduin` over `net/http` (merged with `pkg/server`) |
| DI | Uber Fx | constructor injection in `internal/runtime` |
| Business logic | `models/` | `domains/` (islands) |
| Application logic | `services/` | `services/` (unchanged role) |
| SQL location | `models/queries/` | `domains/<entity>/<entity>.sql` |
| Views | templ+Datastar **or** Inertia | Inertia only |
| Root document | `views/root.templ` | `resources/root.templ` |
| Controllers | `controllers/` | `handlers/` (flat, no sub-packages) |
| Router package | `router/` | `routes/` + `middleware/` (policy only) + `pkg/anduin` |
| Jar | `middleware.NewJar` (exploration) | `internal/runtime.newJar` |
| Runtime | `New` + `NewServer` | single `New` owning `Handler`+`Server` |
| Domain inputs | raw `string` + `Validate()` | parsed newtypes (`ParseEmail`) |
| Factories | `models/factories/` | dropped (inline in tests) |
| Code generation | `generate model/controller/scaffold/…` | `patterns show … [--entity/--table]` |
| Verification | `doctor` only | `verify` (gate) + `doctor` (diagnostics), split |
| Composition | `internal/runtime` FX modules | `internal/runtime` constructors |
| Handler tests | `echo.New()` + `NewContext` | `pkg/anduin/anduintest` |

# Version Two of Andurel

This document describes the initial direction for Andurel V2. It is intended to establish a solid architectural base rather than define every implementation detail.

V2 development happens on `master`. V1 remains maintained on `1-5-stable`.

## Core direction

Andurel V2 should:

- use Uber Fx as the standard dependency injection and lifecycle system;
- move reusable framework functionality into independently versioned Go modules;
- retain Bun as the default model persistence layer and offer sqlc as an opt-in tool for complex queries;
- retain SQL migrations as the source of truth for the database schema;
- make Inertia the recommended option for rich user interfaces while continuing to support templ and Datastar;
- keep application dependencies explicit and testable.

Fx should simplify application wiring, lifecycle management, and route registration. It must not become a service locator, hold request-scoped state, or hide dependencies inside contexts or package globals.

## Standalone framework packages

Reusable framework functionality should live in the Andurel repository under `pkg/`. These packages should be imported by generated applications instead of being copied into each application under `internal/` or `pkg/`.

The recent V2 changes that only move generated package implementations from application `internal/` directories to application `pkg/` directories are an intermediate step, not the intended final architecture.

Initial package candidates include:

```text
pkg/
├── hypermedia/
├── inertia/
├── request/
├── routing/
├── server/
├── storage/
└── validation/
```

Each standalone package should:

- be its own Go module with its own `go.mod`;
- follow its own semantic versioning lifecycle;
- have its own release tags and changelog;
- avoid imports from sibling Andurel modules;
- expose ordinary constructors in addition to any Fx integration;
- provide useful defaults without preventing applications from replacing or extending its behavior.

The Andurel CLI and generator retain the existing module path. V2 is identified by the `v2.0.0` release tag, not by a module-path suffix:

```text
github.com/mbvlabs/andurel
```

Independent packages use paths such as:

```text
github.com/mbvlabs/andurel/pkg/storage
github.com/mbvlabs/andurel/pkg/validation
github.com/mbvlabs/andurel/pkg/inertia
```

A package only adds a major-version suffix when that package introduces a breaking release. Package versions do not need to match the framework version. For example, an Andurel V2 release may use storage `v1.0.2`, validation `v4.0.2`, and Inertia `v1.3.0`.

Each framework release should identify a set of package versions verified together. New applications pin those versions in `go.mod`, which remains the source of truth for application dependencies. Packages can still release and be updated independently between framework releases.

A repository workspace may be used for local development, but each module must also build and verify without relying on workspace-only replacements.

## Application configuration

Application configuration remains in the generated application's root `config/` package. Configuration is application policy and must not become another standalone module under `pkg/`.

Each standalone package owns the configuration type, defaults, validation, and construction options needed for that package. The application config package composes those types and decides how values are loaded and overridden.

For example:

```go
type Config struct {
    App      AppConfig
    Database storage.Config
    Inertia  inertia.Config
}
```

Configuration should be resolved in a predictable order:

```text
package defaults
      ↓
application defaults
      ↓
configuration files or environment variables
      ↓
explicit programmatic overrides
      ↓
validation
      ↓
immutable configuration supplied through Fx
```

This provides the Laravel and Rails style experience of useful framework defaults that applications can override, while preserving Go's typed and explicit configuration model.

Standalone packages should accept configuration values rather than reading application environment variables directly. This keeps environment names, secret sources, and cross-package policy in the application config package.

Configuration should be loaded once during startup, validated before dependent components start, and supplied through Fx. Package-level mutable configuration globals should not be used.

Email infrastructure should be made configurable through the same application-owned pattern used by storage, Inertia, and queue. The email package should expose typed configuration and functional options, while generated application configuration owns environment loading, provider selection, defaults, and Fx wiring. Email implementations should not read environment variables directly.

### Future configuration composition

As a separate future piece of work, consider removing the aggregate `config.Config` type and having Fx constructors receive only the subsystem configuration structs they need. This is explicitly out of scope for the current storage and queue work, which should not broaden into an application-wide configuration injection refactor.

## Storage and Bun

Bun should remain the default persistence layer in V2. It fits Andurel's application-owned model design, keeps ordinary CRUD concise, and remains explicit when queries become SQL-oriented.

GORM was evaluated against representative Andurel models. Rewriting ordinary user CRUD produced almost no meaningful reduction in code or improvement in DX: validation, error normalization, uniqueness checks, pagination, and explicit update fields remained application concerns. Rewriting a complex reporting model had the same string-heavy projections and joins as Bun. GORM's primary advantages—convention-driven associations and its broader ecosystem—would not offset the migration cost for Andurel's current model architecture.

Bob was also evaluated. Its typed ORM query generation is attractive, but its database-first generated models conflict with Andurel's preference for application-owned business entities. Its SQL query generator addresses complex queries, but overlaps with the narrower and more established role sqlc can provide. Neither alternative demonstrated enough benefit to replace Bun.

The standalone storage module should own shared database infrastructure, including:

- PostgreSQL connection creation through `pgx/v5`;
- connection pool defaults and overrides;
- Bun initialization through `pgx/v5/stdlib`;
- access to the underlying `database/sql` pool;
- transaction helpers;
- health checks;
- tracing and logging integration;
- migration execution primitives;
- test database support through a dedicated `storagetest` subpackage.

PostgreSQL remains the initial supported database for V2.

The public database boundary should be a small `storage.Connection` interface:

```go
type Connection interface {
    Executor() bun.IDB
    DB() *sql.DB
}
```

Constructors should return concrete storage types, while application components accept `storage.Connection`. The PostgreSQL implementation should retain its Bun handle internally, use `pgx/v5/stdlib` to create the underlying `*sql.DB`, and expose that same pool through `DB()`. This lets Bun and infrastructure such as River share one pool without making generated applications responsible for connection setup.

Queue infrastructure should use the same connection boundary. The storage package owns the River client adapters and their functional options. Generated configuration has one `queueCfg`, which is translated into those options. Fx exposes separate `QueueInsertModule` and `QueueProcessorModule` values so the web process can enqueue jobs without starting workers, while `cmd/queue/main.go` runs the processor and owns its lifecycle.

### Schema and migrations

SQL migrations remain the canonical database schema history. They should move from `database/migrations/` to a root `migrations/` directory.

Bun model tags and query code must not become a parallel schema definition or migration mechanism. Schema changes continue to begin with SQL migrations.

Seed definitions should move to a root `seeds/` package. Seed data remains application-owned, while reusable execution support may come from the storage module.

### Models and dependency injection

A model should remain a cohesive application concept. Its entity, validation, relationships, domain behavior, constructor, and persistence methods belong together in the model file, such as `models/user.go`. V2 should not introduce a `user_store.go` or repository-per-model convention.

Each model should have a constructor that receives its database dependency. The central `models/models.go` file registers those constructors with Fx, similar to how Andurel packages are composed today:

```go
// models/models.go
package models

import "go.uber.org/fx"

var Module = fx.Module(
    "models",
    fx.Provide(
        NewUsers,
        NewTokens,
    ),
)
```

The model implementation remains in its model file:

```go
// models/user.go
type Users struct {
    db storage.Connection
}

type User struct {
    bun.BaseModel `bun:"table:users,alias:user"`
    ID            uuid.UUID `bun:"id,pk,type:uuid"`
    Email         string    `bun:"email"`
}

func NewUsers(db storage.Connection) Users {
    return Users{db: db}
}

func (users Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
    var user User
    err := users.db.Executor().NewSelect().
        Model(&user).
        Where("user.id = ?", id).
        Scan(ctx)
    return user, err
}

func (user *User) Validate() error {
    // Model validation belongs with the model.
    return nil
}
```

The application composition root includes `models.Module`. Controllers and services receive plural model APIs such as `Users`, while singular types such as `User` represent persisted model records. This avoids repeatedly passing a raw database connection into every model method.

Request cancellation should continue through `context.Context`. The database must not be retrieved from a package global, service locator, or request context.

Transactions should provide both Bun and standard SQL access over the same underlying transaction.

### Optional sqlc support

sqlc should be opt-in and reserved for operations that are materially clearer as standalone SQL, such as complex projections, reporting queries, aggregates, lateral joins, and carefully tuned bulk operations. It is an escape hatch within the model layer, not a second application-wide persistence mode. Most applications and models should never need it.

sqlc queries and generated code should live beneath the models package's `internal` boundary:

```text
models/
├── models.go
├── user.go
└── internal/sqlc/
    ├── queries/
    │   └── users.sql
    └── generated/
```

Only model packages may import the generated sqlc package. Application-owned model and projection types remain the public API. Where sqlc generates database-shaped rows or parameters, the owning model method performs the conversion locally; that mapping cost is accepted only for the uncommon queries where explicit SQL provides a clear maintenance benefit. Controllers and services must not import sqlc-generated packages or expose sqlc-generated types.

Bun and sqlc should use the same connection pool and transaction boundary. The default shared pool should be a `database/sql` handle backed by `pgx/v5/stdlib`. sqlc should generate against the standard SQL interface so enabling it does not create a second PostgreSQL pool.

## User interface direction

Andurel should lean more heavily into Inertia for rich user interfaces without removing templ and Datastar as a server-rendered option.

Interoperability should focus on shared backend behavior such as authentication, authorization, validation, flash messages, route definitions, services, and models. Inertia and templ components are not expected to share the same frontend implementation.

The Andurel-owned Inertia v3 implementation should become an independent package. Application-specific root documents, Vite integration, shared props, and frontend entrypoints remain application-owned.

Inertia SSR remains supported. Production may use a Go-managed JavaScript renderer or an explicitly external renderer. During development, Shadowfax must coordinate the frontend and SSR development processes so only one process manager owns each child process.

The JavaScript package manager and the JavaScript runtime used for SSR are separate concerns and should be represented separately in `andurel.lock`.

## Context direction

HTTP handlers and rendering integrations should use Echo's context at the transport boundary. Services, models, storage, jobs, and other application logic should use the standard library's `context.Context`.

V2 should not introduce a universal Andurel context that attempts to transparently replace both types. Request metadata may use typed standard-context helpers, but dependencies and database handles must not be stored in context.

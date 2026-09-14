# Version Two of Andurel

This document describes the initial direction for Andurel V2. It is intended to establish a solid architectural base rather than define every implementation detail.

V2 development happens on `master`. V1 remains maintained on `1-5-stable`.

There is no upgrade path from V1 to V2. V2 applications are created with `andurel new`; existing V1 projects stay on `1-5-stable` until they are rebuilt or migrated manually outside Andurel's automated tooling.

## Core direction

Andurel V2 should:

- use Go 1.27 as its minimum supported Go version;
- use Uber Fx as the standard dependency injection and lifecycle system;
- move reusable framework functionality into independently versioned Go modules;
- persist models through narsilc-generated queries into application-owned structs;
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
├── routing/
├── server/
├── storage/
└── validation/
```

The former `pkg/request` module was not kept. Request-scoped metadata such as flash messages travels through typed helpers in the application-owned `router/appctx` package instead.

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

Generated applications expose one Fx provider per subsystem configuration type, such as `config.AppCfg`, `config.Database`, and `config.QueueCfg`, rather than a single aggregate `config.Config` struct.

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

## Storage and narsilc

V2 persists through narsilc only. There is no Bun default and no sqlc escape hatch. Application-owned model structs carry `andurel:"column"` tags; narsilc generic methods scan into those structs.

The standalone storage module should own shared database infrastructure, including:

- PostgreSQL connection creation through `pgx/v5`;
- connection pool defaults and overrides;
- access to the underlying `database/sql` pool;
- transaction helpers;
- health checks;
- tracing and logging integration;
- migration execution primitives;
- test database support through helpers such as `TestCluster` and `RunMigrations` in the storage module itself, not a separate `storagetest` subpackage.

PostgreSQL remains the initial supported database for V2.

The public database boundary should be a small `storage.Connection` interface:

```go
type Connection interface {
    Health(ctx context.Context) error
    BeginTransaction(ctx context.Context) (Transaction, error)
}
```

Constructors should return concrete storage types, while application components accept `storage.Connection`. The PostgreSQL implementation wraps a `pgx/v5` pool. Models construct narsilc clients with `queries.New(db)` because `Connection` implements pgx DBTX. River uses `riverpgxv5` on that same pool.

Queue infrastructure should use the same connection boundary. The storage package owns the River client adapters and their functional options. Generated configuration has one `queueCfg`, which is translated into those options. Fx exposes separate `QueueInsertModule` and `QueueProcessorModule` values so the web process can enqueue jobs without starting workers, while `cmd/queue/main.go` runs the processor and owns its lifecycle.

### Schema and migrations

SQL migrations remain the canonical database schema history. Generated applications store them in a root `migrations/` directory. The `migrations` package embeds those SQL files for tests and tooling:

```text
migrations/
├── migrations.go   // package migrations, //go:embed *.sql
└── *.sql
```

Model tags and generated query code must not become a parallel schema definition or migration mechanism. Schema changes continue to begin with SQL migrations.

### Seeds and factories

Seed orchestration and model factories remain separate concerns:

```text
seeds/                  # named seed sets and registry (development, test, ...)
models/factories/       # per-model builders for tests and seeds
cmd/seeds/              # CLI entrypoint for andurel database seed
```

The root `seeds/` package owns environment-specific compositions and the registry consumed by `andurel database seed`. It calls into `models/factories/` but factories do not import seeds. Factories stay beside the models the generator owns so factory sync continues to track model changes. Seeds and factories should not be merged into one package.

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
    queries *queries.Queries
}

type User struct {
    ID    uuid.UUID `andurel:"id"`
    Email string    `andurel:"email"`
}

func NewUsers(db storage.Connection) Users {
    return Users{queries: queries.New(db)}
}

func (users Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
    return users.queries.GetUser[User](ctx, id)
}

func (user *User) Validate() error {
    // Model validation belongs with the model.
    return nil
}
```

The application composition root includes `models.Module`. Controllers and services receive plural model APIs such as `Users`, while singular types such as `User` represent persisted model records. This avoids repeatedly passing a raw database connection into every model method.

Request cancellation should continue through `context.Context`. The database must not be retrieved from a package global, service locator, or request context.

Transactions share one `storage.Connection` / `storage.Transaction` boundary. `BeginTransaction` returns a `storage.Transaction` that narsilc accepts with `queries.New(tx)` and that River inserts can share. `storage.RunInTransaction` wraps commit and rollback for multi-step workflows.

### narsilc

narsilc is the only persistence compiler. Since narsilc v0.3.0, scaffolding does not write a `narsilc.yaml`; narsilc reads the project's `andurel.lock` (schema, queries, output package, `sql_package: pgx/v5`, and `row_mapping: andurel`). Query files and generated code use this layout:

```text
andurel.lock            # narsilc reads config from the project lock file
models/
├── models.go
├── user.go
├── queries/              # hand-written SQL query files
└── internal/
    └── queries/          # narsilc-generated Go code
```

Only model packages may import the generated queries package. Application-owned model and projection types remain the public API. Controllers and services must not import generated packages or expose generated types.

The generator writes SQL query files for each model and runs narsilc after `andurel generate model`. Use `andurel generate query` for additional query groups and `andurel generate queries` to regenerate typed code. `andurel run`, `andurel build`, scaffold, and extension apply regenerate narsilc output when annotated query files exist.

## User interface direction

Andurel should lean more heavily into Inertia for rich user interfaces without removing templ and Datastar as a server-rendered option.

Interoperability should focus on shared backend behavior such as authentication, authorization, validation, flash messages, route definitions, services, and models. Inertia and templ components are not expected to share the same frontend implementation.

The Andurel-owned Inertia v3 implementation should become an independent package. Application-specific root documents, Vite integration, shared props, and frontend entrypoints remain application-owned.

Inertia SSR remains supported. Production may use a Go-managed JavaScript renderer or an explicitly external renderer. During development, Shadowfax must coordinate the frontend and SSR development processes so only one process manager owns each child process.

The JavaScript package manager and the JavaScript runtime used for SSR are separate concerns and should be represented separately in `andurel.lock`.

## Context direction

HTTP handlers and rendering integrations should use Echo's context at the transport boundary. Services, models, storage, jobs, and other application logic should use the standard library's `context.Context`.

V2 should not introduce a universal Andurel context that attempts to transparently replace both types. Request metadata may use typed standard-context helpers, but dependencies and database handles must not be stored in context.

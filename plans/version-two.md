# Version Two of Andurel

This document describes the initial direction for Andurel V2. It is intended to establish a solid architectural base rather than define every implementation detail.

V2 development happens on `master`. V1 remains maintained on `1-5-stable`.

## Core direction

Andurel V2 should:

- use Uber Fx as the standard dependency injection and lifecycle system;
- move reusable framework functionality into independently versioned Go modules;
- use GORM as the default model persistence layer;
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

## Storage and GORM

GORM should replace Bun as the default persistence layer in V2. It better supports the intended MVC workflow and should make ordinary model relationships and CRUD operations faster to develop. Generated model code should use GORM's generic API rather than its legacy untyped query API.

The standalone storage module should own shared database infrastructure, including:

- PostgreSQL connection creation through `pgx/v5`;
- connection pool defaults and overrides;
- GORM initialization through the PostgreSQL driver backed by `pgx/v5/stdlib`;
- support for GORM's generic API;
- access to the underlying `database/sql` pool;
- transaction helpers;
- health checks;
- tracing and logging integration;
- migration execution primitives;
- database error normalization;
- test database support through a dedicated `storagetest` subpackage.

PostgreSQL remains the initial supported database for V2.

### Schema and migrations

SQL migrations remain the canonical database schema history. They should move from `database/migrations/` to a root `migrations/` directory.

GORM `AutoMigrate` must not be used as the production migration strategy. It may be made available for explicitly selected disposable development or test workflows, but it must not compete with SQL migrations as a second source of truth.

The GORM CLI should be evaluated after the core model and migration workflow is established. It should only be adopted where it supports the canonical SQL migration workflow rather than creating a parallel schema definition.

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
    db *storage.DB
}

type User struct {
    ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
    Email string    `gorm:"uniqueIndex;not null"`
}

func NewUsers(db *storage.DB) Users {
    return Users{db: db}
}

func (users Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
    user, err := gorm.G[User](users.db.GORM()).
        Where("id = ?", id).
        First(ctx)
    return user, storage.NormalizeError(err)
}

func (user *User) Validate() error {
    // Model validation belongs with the model.
    return nil
}
```

The application composition root includes `models.Module`. Controllers and services receive plural model APIs such as `Users`, while singular types such as `User` represent persisted model records. This avoids repeatedly passing a raw database connection into every model method.

Request cancellation should continue through `context.Context`. The database must not be retrieved from a package global, service locator, or request context.

Transactions should provide both GORM and standard SQL access over the same underlying transaction.

### Optional sqlc support

sqlc should be opt-in and selected for operations where explicit SQL and generated query types are valuable. It is an escape hatch within the model layer, not a second application-wide persistence mode.

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

Only model packages may import the generated sqlc package. Model methods must convert generated rows and parameters to application-owned model types. Controllers and services must not import sqlc-generated packages or expose sqlc-generated types in their APIs.

GORM and sqlc should use the same connection pool and transaction boundary. The default shared pool should be a `database/sql` handle backed by `pgx/v5/stdlib`. sqlc may use that standard SQL interface so enabling it does not create a second PostgreSQL pool.

## User interface direction

Andurel should lean more heavily into Inertia for rich user interfaces without removing templ and Datastar as a server-rendered option.

Interoperability should focus on shared backend behavior such as authentication, authorization, validation, flash messages, route definitions, services, and models. Inertia and templ components are not expected to share the same frontend implementation.

The Andurel-owned Inertia v3 implementation should become an independent package. Application-specific root documents, Vite integration, shared props, and frontend entrypoints remain application-owned.

Inertia SSR remains supported. Production may use a Go-managed JavaScript renderer or an explicitly external renderer. During development, Shadowfax must coordinate the frontend and SSR development processes so only one process manager owns each child process.

The JavaScript package manager and the JavaScript runtime used for SSR are separate concerns and should be represented separately in `andurel.lock`.

## Context direction

HTTP handlers and rendering integrations should use Echo's context at the transport boundary. Services, models, storage, jobs, and other application logic should use the standard library's `context.Context`.

V2 should not introduce a universal Andurel context that attempts to transparently replace both types. Request metadata may use typed standard-context helpers, but dependencies and database handles must not be stored in context.

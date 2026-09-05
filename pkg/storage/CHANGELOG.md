# Changelog

All notable changes to the standalone Andurel storage module are documented here.

## 0.6.0 - 2026-09-05

### Added

- `DefaultConfig` for operational PostgreSQL defaults without application credentials.
- `DefaultQueueConfig`, application-owned `QueueConfig.Queues`, and `QueueConfig.Clone`.

### Changed

- `NewPostgres` takes `Config` directly. Remaining options override individual connection settings.
- `NewPostgres` preserves pgx URL settings unless explicitly overridden and selects a compatible query execution mode when a cache is disabled.
- `Config.Validate` rejects unsupported database kinds and SSL modes.
- `NewQueueInsert` and `NewQueueProcessor` take `QueueConfig` and validate it before applying River options.
- Queue validation follows River defaults, the infinite-duration sentinel, queue limits, and timing constraints.

### Removed

- `WithConfig` and `WithQueueConfig`.

## 0.5.0 - 2026-09-02

### Added

- Typed `QueueConfig` with validation and conversion to River configuration.
- `WithQueueConfig` for applying application-owned queue settings.

### Removed

- Package-level database default constants and `DefaultConfig`. Applications supply configuration values.

## 0.4.0 - 2026-09-01

### Added

- Canonical `sqlc.yaml` embedded in the storage module for always-available sqlc integration.
- Helpers to write the sqlc config during scaffolding and detect query files in `models/queries/`.
- `storage.Transaction`, `Connection.BeginTransaction`, and `storage.RunInTransaction` for shared Bun and `database/sql` transaction boundaries.

### Changed

- `Connection` now requires `BeginTransaction`; the concrete `Postgres.BeginTx` helper returning `bun.Tx` was removed.
- `HasSQLCQueryFiles` ignores `.sql` stubs that do not contain sqlc query annotations.

## 0.3.0 - 2026-08-26

### Added

- Concrete insert-only and processor River clients backed by the `database/sql` pool exposed by `Connection`.
- Functional options for configuring River clients, workers, queues, periodic jobs, middleware, hooks, logging, retry behavior, and lifecycle settings.
- Start and stop lifecycle methods on the concrete queue processor.

## 0.2.0 - 2026-08-26

### Added

- Typed PostgreSQL configuration with development defaults and functional options.
- Connection pool, pgx runtime parameter, TLS, and OpenTelemetry configuration.
- The `Connection` interface for accessing Bun through `Executor()` and the shared `database/sql` pool through `DB()`.

### Changed

- `NewPostgres` now accepts functional options and returns the concrete `*Postgres` implementation.
- Replaced the `Pool` interface with the smaller `Connection` interface.
- Renamed `Conn()` to `DB()` and generalized `Executor()` from `*bun.DB` to `bun.IDB`.
- `Postgres` now retains its Bun handle explicitly while continuing to create the underlying pool through `pgx/v5/stdlib`.

## 0.1.0 - 2026-08-20

### Added

- Initial standalone storage module with PostgreSQL, Bun, migrations, queue insertion contracts, and test database support.

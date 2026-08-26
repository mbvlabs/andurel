# Changelog

All notable changes to the standalone Andurel storage module are documented here.

## Unreleased

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

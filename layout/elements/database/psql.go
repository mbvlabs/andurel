package database

import (
	"context"
	"embed"
	"errors"
	"log/slog"

	"mbvlabs/andurel/layout/elements/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBeginTx    = errors.New("could not begin transaction")
	ErrRollbackTx = errors.New("could not rollback transaction")
	ErrCommitTx   = errors.New("could not commit transaction")
)

//go:embed migrations/*.sql
var Migrations embed.FS

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context) (Postgres, error) {
	cfg, err := pgxpool.ParseConfig(config.DB.GetDatabaseURL())
	if err != nil {
		slog.ErrorContext(ctx, "could not parse database connection string", "error", err)
		return Postgres{}, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.ErrorContext(ctx, "could not establish connection to database", "error", err)
		return Postgres{}, err
	}

	if err := pool.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "could not ping database", "error", err)
		return Postgres{}, err
	}

	return Postgres{pool}, nil
}

func (p *Postgres) Pool() *pgxpool.Pool {
	return p.pool
}

func (p *Postgres) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not begin transaction", "reason", err)
		return nil, errors.Join(ErrBeginTx, err)
	}

	return tx, nil
}

func (p *Postgres) RollBackTx(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Rollback(ctx); err != nil {
		slog.ErrorContext(ctx, "could not rollback transaction", "reason", err)
		return errors.Join(ErrRollbackTx, err)
	}

	return nil
}

func (p *Postgres) CommitTx(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		slog.ErrorContext(ctx, "could not commit transaction", "reason", err)
		return errors.Join(ErrCommitTx, err)
	}

	return nil
}

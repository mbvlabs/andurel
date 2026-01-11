// Package storage provides abstractions for database interactions and default implementations.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBeginTx    = errors.New("could not begin transaction")
	ErrRollbackTx = errors.New("could not rollback transaction")
	ErrCommitTx   = errors.New("could not commit transaction")
)

type Executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Pool interface {
	Conn() *pgxpool.Pool
	BeginTx(ctx context.Context) (pgx.Tx, error)
	RollBackTx(ctx context.Context, tx pgx.Tx) error
	CommitTx(ctx context.Context, tx pgx.Tx) error
}

// Connection wraps a pgxpool.Pool for simple database access.
type Connection struct {
	pool *pgxpool.Pool
}

// NewConnection creates a new database connection pool.
func NewConnection(ctx context.Context, databaseURL string) (*Connection, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{pool: pool}, nil
}

// Conn returns the underlying pool which implements Executor.
func (c *Connection) Conn() *pgxpool.Pool {
	return c.pool
}

// Close closes the connection pool.
func (c *Connection) Close() {
	c.pool.Close()
}

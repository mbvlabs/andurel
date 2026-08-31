package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

// Transaction exposes Bun and database/sql access over one PostgreSQL
// transaction. Use Executor for Bun model methods and SQL for sqlc, River
// inserts, or other database/sql callers that must share the same boundary.
type Transaction interface {
	Executor() bun.IDB
	SQL() *sql.Tx
	Commit() error
	Rollback() error
}

type bunTransaction struct {
	tx bun.Tx
}

func (t bunTransaction) Executor() bun.IDB {
	return t.tx
}

func (t bunTransaction) SQL() *sql.Tx {
	return t.tx.Tx
}

func (t bunTransaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit transaction: %w", err)
	}
	return nil
}

func (t bunTransaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("storage: rollback transaction: %w", err)
	}
	return nil
}

// RunInTransaction runs fn inside a transaction and commits on success.
func RunInTransaction(
	ctx context.Context,
	conn Connection,
	opts *sql.TxOptions,
	fn func(context.Context, Transaction) error,
) error {
	tx, err := conn.BeginTransaction(ctx, opts)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	committed = true
	return nil
}

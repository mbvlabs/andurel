package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Transaction is one PostgreSQL transaction on a Connection. narsilc
// clients accept it directly: queries.New(tx). River inserts that must
// share the boundary use InsertTx with this same value.
type Transaction interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type pgxTransaction struct {
	tx pgx.Tx
}

func (t pgxTransaction) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, arguments...)
}

func (t pgxTransaction) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t pgxTransaction) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t pgxTransaction) pgxTx() pgx.Tx {
	return t.tx
}

func (t pgxTransaction) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("storage: commit transaction: %w", err)
	}
	return nil
}

func (t pgxTransaction) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		return fmt.Errorf("storage: rollback transaction: %w", err)
	}
	return nil
}

// RunInTransaction runs fn inside a transaction and commits on success.
func RunInTransaction(
	ctx context.Context,
	conn Connection,
	fn func(context.Context, Transaction) error,
) error {
	tx, err := conn.BeginTransaction(ctx)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	committed = true
	return nil
}

type pgxTxProvider interface {
	pgxTx() pgx.Tx
}

func pgxTxFrom(tx Transaction) (pgx.Tx, error) {
	provider, ok := tx.(pgxTxProvider)
	if !ok {
		return nil, fmt.Errorf("storage: transaction does not expose a PostgreSQL tx")
	}
	pgxTx := provider.pgxTx()
	if pgxTx == nil {
		return nil, fmt.Errorf("storage: transaction returned a nil tx")
	}
	return pgxTx, nil
}

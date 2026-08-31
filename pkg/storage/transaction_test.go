package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/uptrace/bun"
)

type stubConnection struct {
	begin func(context.Context, *sql.TxOptions) (Transaction, error)
}

func (s stubConnection) Executor() bun.IDB { return nil }
func (s stubConnection) DB() *sql.DB       { return nil }
func (s stubConnection) BeginTransaction(
	ctx context.Context,
	opts *sql.TxOptions,
) (Transaction, error) {
	return s.begin(ctx, opts)
}

type stubTransaction struct {
	commit   func() error
	rollback func() error
}

func (s stubTransaction) Executor() bun.IDB { return nil }
func (s stubTransaction) SQL() *sql.Tx      { return nil }
func (s stubTransaction) Commit() error {
	if s.commit == nil {
		return nil
	}
	return s.commit()
}
func (s stubTransaction) Rollback() error {
	if s.rollback == nil {
		return nil
	}
	return s.rollback()
}

func TestRunInTransactionCommitsOnSuccess(t *testing.T) {
	committed := false
	rolledBack := false

	conn := stubConnection{
		begin: func(context.Context, *sql.TxOptions) (Transaction, error) {
			return stubTransaction{
				commit: func() error {
					committed = true
					return nil
				},
				rollback: func() error {
					rolledBack = true
					return nil
				},
			}, nil
		},
	}

	if err := RunInTransaction(context.Background(), conn, nil, func(context.Context, Transaction) error {
		return nil
	}); err != nil {
		t.Fatalf("RunInTransaction: %v", err)
	}
	if !committed {
		t.Fatal("expected commit")
	}
	if rolledBack {
		t.Fatal("did not expect rollback after successful commit")
	}
}

func TestRunInTransactionRollsBackOnError(t *testing.T) {
	committed := false
	rolledBack := false

	conn := stubConnection{
		begin: func(context.Context, *sql.TxOptions) (Transaction, error) {
			return stubTransaction{
				commit: func() error {
					committed = true
					return nil
				},
				rollback: func() error {
					rolledBack = true
					return nil
				},
			}, nil
		},
	}

	err := RunInTransaction(context.Background(), conn, nil, func(context.Context, Transaction) error {
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if committed {
		t.Fatal("did not expect commit")
	}
	if !rolledBack {
		t.Fatal("expected rollback")
	}
}

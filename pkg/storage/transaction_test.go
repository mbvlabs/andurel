package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubConnection struct {
	begin func(context.Context) (Transaction, error)
}

func (stubConnection) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not implemented")
}
func (stubConnection) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (stubConnection) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
func (stubConnection) Health(context.Context) error { return nil }
func (s stubConnection) BeginTransaction(ctx context.Context) (Transaction, error) {
	return s.begin(ctx)
}

type stubTransaction struct {
	commit   func(context.Context) error
	rollback func(context.Context) error
}

func (stubTransaction) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not implemented")
}
func (stubTransaction) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (stubTransaction) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
func (s stubTransaction) Commit(ctx context.Context) error {
	if s.commit == nil {
		return nil
	}
	return s.commit(ctx)
}
func (s stubTransaction) Rollback(ctx context.Context) error {
	if s.rollback == nil {
		return nil
	}
	return s.rollback(ctx)
}

func TestRunInTransactionCommitsOnSuccess(t *testing.T) {
	committed := false
	rolledBack := false

	conn := stubConnection{
		begin: func(context.Context) (Transaction, error) {
			return stubTransaction{
				commit: func(context.Context) error {
					committed = true
					return nil
				},
				rollback: func(context.Context) error {
					rolledBack = true
					return nil
				},
			}, nil
		},
	}

	if err := RunInTransaction(context.Background(), conn, func(context.Context, Transaction) error {
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
		begin: func(context.Context) (Transaction, error) {
			return stubTransaction{
				commit: func(context.Context) error {
					committed = true
					return nil
				},
				rollback: func(context.Context) error {
					rolledBack = true
					return nil
				},
			}, nil
		},
	}

	err := RunInTransaction(context.Background(), conn, func(context.Context, Transaction) error {
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

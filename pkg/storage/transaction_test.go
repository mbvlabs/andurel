package storage

import (
	"context"
	"errors"
	"strings"
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

	if err := runInTransaction(context.Background(), conn, func(context.Context, Transaction) error {
		return nil
	}); err != nil {
		t.Fatalf("runInTransaction: %v", err)
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

	err := runInTransaction(context.Background(), conn, func(context.Context, Transaction) error {
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

type unwrapStub struct {
	stubTransaction
	next Transaction
}

func (u unwrapStub) UnwrapTransaction() Transaction { return u.next }

type pgxProviderStub struct {
	stubTransaction
	tx pgx.Tx
}

func (p pgxProviderStub) pgxTx() pgx.Tx { return p.tx }

func TestPGXTxFromRejectsNilAndNonPGXTransactions(t *testing.T) {
	tests := []struct {
		name string
		tx   Transaction
	}{
		{name: "nil", tx: nil},
		{name: "plain", tx: stubTransaction{}},
		{name: "nil provider result", tx: pgxProviderStub{}},
		{name: "unwrap to nil", tx: unwrapStub{next: nil}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := pgxTxFrom(test.tx); err == nil {
				t.Fatal("pgxTxFrom succeeded, want an error")
			}
		})
	}
}

func TestPGXTxFromStopsUnwrapCycles(t *testing.T) {
	first := &unwrapStub{}
	second := &unwrapStub{}
	first.next = second
	second.next = first

	_, err := pgxTxFrom(first)
	if err == nil {
		t.Fatal("pgxTxFrom succeeded on a cycle, want an error")
	}
	if !strings.Contains(err.Error(), "unwrap depth exceeded") {
		t.Fatalf("error = %v, want the unwrap depth error", err)
	}
}

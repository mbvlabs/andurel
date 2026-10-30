package storagetest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type faultStubConnection struct {
	begin            func(context.Context) (storage.Transaction, error)
	beginWithOptions func(context.Context, pgx.TxOptions) (storage.Transaction, error)
}

func (c faultStubConnection) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected direct Exec")
}

func (c faultStubConnection) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected direct Query")
}

func (c faultStubConnection) QueryRow(context.Context, string, ...any) pgx.Row {
	return faultStubRow{err: errors.New("unexpected direct QueryRow")}
}

func (c faultStubConnection) Health(context.Context) error {
	return nil
}

func (c faultStubConnection) BeginTransaction(ctx context.Context) (storage.Transaction, error) {
	return c.begin(ctx)
}

func (c faultStubConnection) BeginTransactionWithOptions(
	ctx context.Context,
	options pgx.TxOptions,
) (storage.Transaction, error) {
	return c.beginWithOptions(ctx, options)
}

type faultStubTransaction struct {
	commits   int
	rollbacks int
}

func (t *faultStubTransaction) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *faultStubTransaction) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *faultStubTransaction) QueryRow(context.Context, string, ...any) pgx.Row {
	return faultStubRow{}
}

func (t *faultStubTransaction) Commit(context.Context) error {
	t.commits++

	return nil
}

func (t *faultStubTransaction) Rollback(context.Context) error {
	t.rollbacks++

	return nil
}

type faultStubRow struct {
	err error
}

func (r faultStubRow) Scan(...any) error {
	return r.err
}

func faultStubTransactor(tx *faultStubTransaction, fault StatementFault) storage.Transactor {
	return FailingTransactor(faultStubConnection{
		begin: func(context.Context) (storage.Transaction, error) {
			return tx, nil
		},
	}, fault)
}

func TestFailingTransactorFailsNthStatement(t *testing.T) {
	tx := &faultStubTransaction{}
	transactor := faultStubTransactor(tx, StatementFault{FailAt: 2})

	err := storage.AtomicOps(t.Context(), transactor, func(ctx context.Context, tx storage.Tx) error {
		statements := storage.Unwrap(tx)
		if _, err := statements.Exec(ctx, "first"); err != nil {
			return err
		}

		_, err := statements.Exec(ctx, "second")

		return err
	})
	if !errors.Is(err, ErrFaultInjected) {
		t.Fatalf("AtomicOps error = %v, want %v", err, ErrFaultInjected)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d, want 0/1", tx.commits, tx.rollbacks)
	}
}

func TestFailingTransactorMatchesStatements(t *testing.T) {
	tx := &faultStubTransaction{}
	transactor := faultStubTransactor(tx, StatementFault{
		FailAt: 1,
		Match: func(query string) bool {
			return strings.Contains(query, "audit")
		},
	})

	err := storage.AtomicOps(t.Context(), transactor, func(ctx context.Context, tx storage.Tx) error {
		statements := storage.Unwrap(tx)
		if _, err := statements.Exec(ctx, "INSERT INTO widgets"); err != nil {
			return err
		}

		if _, err := statements.Exec(ctx, "INSERT INTO audit_log"); !errors.Is(err, ErrFaultInjected) {
			return errors.New("audit statement did not fail")
		}

		_, err := statements.Exec(ctx, "INSERT INTO widgets")

		return err
	})
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("commits=%d rollbacks=%d, want 1/0", tx.commits, tx.rollbacks)
	}
}

func TestFailingTransactorFailsQuery(t *testing.T) {
	tx := &faultStubTransaction{}
	transactor := faultStubTransactor(tx, StatementFault{})

	err := storage.AtomicOps(t.Context(), transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := storage.Unwrap(tx).Query(ctx, "SELECT 1")

		return err
	})
	if !errors.Is(err, ErrFaultInjected) {
		t.Fatalf("AtomicOps error = %v, want %v", err, ErrFaultInjected)
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollbacks = %d, want 1", tx.rollbacks)
	}
}

func TestFailingTransactorFailsQueryRowScan(t *testing.T) {
	tx := &faultStubTransaction{}
	transactor := faultStubTransactor(tx, StatementFault{})

	err := storage.AtomicOps(t.Context(), transactor, func(ctx context.Context, tx storage.Tx) error {
		var value int

		return storage.Unwrap(tx).QueryRow(ctx, "SELECT 1").Scan(&value)
	})
	if !errors.Is(err, ErrFaultInjected) {
		t.Fatalf("AtomicOps error = %v, want %v", err, ErrFaultInjected)
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollbacks = %d, want 1", tx.rollbacks)
	}
}

func TestFailingTransactorUsesCustomError(t *testing.T) {
	tx := &faultStubTransaction{}
	sentinel := errors.New("late failure")
	transactor := faultStubTransactor(tx, StatementFault{Err: sentinel})

	err := storage.AtomicOps(t.Context(), transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := storage.Unwrap(tx).Exec(ctx, "INSERT INTO widgets")

		return err
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("AtomicOps error = %v, want %v", err, sentinel)
	}
	if errors.Is(err, ErrFaultInjected) {
		t.Fatalf("AtomicOps error = %v, want only the custom error", err)
	}
}

func TestFailingTransactorAppliesTransactionOptions(t *testing.T) {
	tx := &faultStubTransaction{}
	var got pgx.TxOptions
	transactor := FailingTransactor(faultStubConnection{
		beginWithOptions: func(_ context.Context, options pgx.TxOptions) (storage.Transaction, error) {
			got = options

			return tx, nil
		},
	}, StatementFault{})

	err := storage.AtomicOps(t.Context(), transactor, func(context.Context, storage.Tx) error {
		return nil
	}, storage.ReadOnly())
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if got.AccessMode != pgx.ReadOnly {
		t.Fatalf("access mode = %q, want %q", got.AccessMode, pgx.ReadOnly)
	}
}

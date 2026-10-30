package storagetest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// ErrFaultInjected is returned by a transaction statement selected by
// StatementFault when StatementFault.Err is nil.
var ErrFaultInjected = errors.New("storagetest: injected statement fault")

// StatementFault selects the transaction statement that FailingTransactor
// fails. FailAt is the 1-based number of a matching statement; values below 1
// fail the first matching statement. Match selects statements by their SQL
// text, and nil matches every statement. Err replaces ErrFaultInjected as the
// returned error.
type StatementFault struct {
	FailAt int
	Match  func(query string) bool
	Err    error
}

// FailingTransactor returns a transactor whose transactions fail one selected
// statement before it reaches PostgreSQL. Pass the returned Transactor to the
// service under test: AtomicOps and the service see the injected error and roll
// back everything written before it, so a rollback test does not need an
// unrelated domain failure.
func FailingTransactor(conn storage.Connection, fault StatementFault) storage.Transactor {
	return storage.NewTransactor(failingConnection{
		Connection: conn,
		fault:      fault,
	})
}

type failingConnection struct {
	storage.Connection
	fault StatementFault
}

type transactionOptionsBeginner interface {
	BeginTransactionWithOptions(context.Context, pgx.TxOptions) (storage.Transaction, error)
}

func (c failingConnection) BeginTransaction(ctx context.Context) (storage.Transaction, error) {
	tx, err := c.Connection.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return c.failing(tx), nil
}

func (c failingConnection) BeginTransactionWithOptions(
	ctx context.Context,
	options pgx.TxOptions,
) (storage.Transaction, error) {
	beginner, ok := c.Connection.(transactionOptionsBeginner)
	if !ok {
		return nil, fmt.Errorf("storagetest: connection does not support transaction options")
	}

	tx, err := beginner.BeginTransactionWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	return c.failing(tx), nil
}

func (c failingConnection) failing(tx storage.Transaction) storage.Transaction {
	return &failingTransaction{
		Transaction: tx,
		fault:       c.fault,
	}
}

type failingTransaction struct {
	storage.Transaction
	fault   StatementFault
	matched int
}

func (t *failingTransaction) Exec(
	ctx context.Context,
	query string,
	args ...any,
) (pgconn.CommandTag, error) {
	if err := t.faultError(query); err != nil {
		return pgconn.CommandTag{}, err
	}

	return t.Transaction.Exec(ctx, query, args...)
}

func (t *failingTransaction) Query(
	ctx context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	if err := t.faultError(query); err != nil {
		return nil, err
	}

	return t.Transaction.Query(ctx, query, args...)
}

func (t *failingTransaction) QueryRow(
	ctx context.Context,
	query string,
	args ...any,
) pgx.Row {
	if err := t.faultError(query); err != nil {
		return errorRow{err: err}
	}

	return t.Transaction.QueryRow(ctx, query, args...)
}

func (t *failingTransaction) faultError(query string) error {
	if t.fault.Match != nil && !t.fault.Match(query) {
		return nil
	}

	t.matched++
	failAt := max(t.fault.FailAt, 1)
	if t.matched != failAt {
		return nil
	}

	if t.fault.Err != nil {
		return t.fault.Err
	}

	return ErrFaultInjected
}

type errorRow struct {
	err error
}

func (r errorRow) Scan(...any) error {
	return r.err
}

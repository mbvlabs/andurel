package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// IsolationLevel names a PostgreSQL transaction isolation level.
type IsolationLevel string

const (
	ReadUncommitted IsolationLevel = IsolationLevel(pgx.ReadUncommitted)
	ReadCommitted   IsolationLevel = IsolationLevel(pgx.ReadCommitted)
	RepeatableRead  IsolationLevel = IsolationLevel(pgx.RepeatableRead)
	Serializable    IsolationLevel = IsolationLevel(pgx.Serializable)
)

// TxOption configures the transaction AtomicOps opens.
type TxOption interface {
	apply(*txOptions)
}

type txOptionFunc func(*txOptions)

func (f txOptionFunc) apply(options *txOptions) {
	f(options)
}

type txOptions struct {
	isolation  pgx.TxIsoLevel
	accessMode pgx.TxAccessMode
}

// ReadOnly opens the transaction with PostgreSQL's READ ONLY access mode, so
// any write fails instead of committing.
func ReadOnly() TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.accessMode = pgx.ReadOnly
	})
}

// Isolation opens the transaction at level. The default is the server's
// setting, normally READ COMMITTED.
func Isolation(level IsolationLevel) TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.isolation = pgx.TxIsoLevel(level)
	})
}

func (o txOptions) isZero() bool {
	return o.isolation == "" && o.accessMode == ""
}

func (o txOptions) pgx() pgx.TxOptions {
	return pgx.TxOptions{
		IsoLevel:   o.isolation,
		AccessMode: o.accessMode,
	}
}

func (o txOptions) validate() error {
	switch o.isolation {
	case "",
		pgx.ReadUncommitted,
		pgx.ReadCommitted,
		pgx.RepeatableRead,
		pgx.Serializable:
		return nil
	default:
		return fmt.Errorf("storage: unsupported isolation level %q", o.isolation)
	}
}

type txOptionsBeginner interface {
	BeginTransactionWithOptions(context.Context, pgx.TxOptions) (Transaction, error)
}

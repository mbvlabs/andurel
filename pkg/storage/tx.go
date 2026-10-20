package storage

import "context"

// Tx is an open transaction. It has no query methods: holding one lets a
// caller bind to the transaction (an InTx method) but not issue SQL. Only
// AtomicOps creates a usable Tx.
type Tx struct {
	tx Transaction
}

// Transactor opens transactions for AtomicOps. One from NewTransactor opens
// a new transaction per call; one returned by InTx joins that transaction
// instead, so code that is atomic on its own also composes into a caller's
// transaction.
type Transactor struct {
	conn  Connection
	bound Tx
}

// NewTransactor returns a Transactor that opens transactions on conn.
func NewTransactor(conn Connection) Transactor {
	return Transactor{conn: conn}
}

// InTx returns a Transactor whose AtomicOps joins tx instead of opening a
// new transaction.
func (t Transactor) InTx(tx Tx) Transactor {
	return Transactor{conn: t.conn, bound: tx}
}

// AtomicOps runs fn inside one transaction, and is the only way to commit
// or roll one back.
//
// With an unbound Transactor it opens the transaction, commits when fn
// returns nil, and rolls back when fn returns an error or panics. The error
// is returned unchanged; a commit failure is returned wrapped.
//
// With a bound Transactor (see InTx) it runs fn in the bound transaction and
// leaves commit and rollback to the owner of that transaction.
func AtomicOps(
	ctx context.Context,
	t Transactor,
	fn func(ctx context.Context, tx Tx) error,
) error {
	if t.bound.tx != nil {
		return fn(ctx, t.bound)
	}

	return runInTransaction(ctx, t.conn, func(ctx context.Context, tx Transaction) error {
		return fn(ctx, Tx{tx: tx})
	})
}

// Unwrap returns the transaction behind tx so a domain receiver can bind its
// generated queries to it (queries.New(storage.Unwrap(tx))). Only the
// domains layer calls it; andurel verify fails on any other call site.
func Unwrap(tx Tx) Transaction {
	if tx.tx == nil {
		panic("storage: zero Tx; obtain one from AtomicOps")
	}

	return tx.tx
}

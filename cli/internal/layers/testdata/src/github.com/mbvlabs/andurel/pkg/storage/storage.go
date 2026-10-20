package storage

import "context"

type Connection interface {
	Exec(ctx context.Context, sql string, arguments ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
	QueryRow(ctx context.Context, sql string, args ...any) any
	Health(ctx context.Context) error
	BeginTransaction(ctx context.Context) (Transaction, error)
}

type Tx struct{}

type Transaction interface{}

type Transactor struct{}

func NewTransactor(conn Connection) Transactor { return Transactor{} }

func (t Transactor) InTx(tx Tx) Transactor { return t }

func AtomicOps(ctx context.Context, t Transactor, fn func(ctx context.Context, tx Tx) error) error {
	return fn(ctx, Tx{})
}

func Unwrap(tx Tx) Transaction { return nil }

type InsertQueue interface {
	Insert(ctx context.Context, args any, opts any) (any, error)
	InTx(tx Tx) InsertQueue
}

type PoolHandle struct{}

func (p *PoolHandle) Exec(ctx context.Context, sql string, arguments ...any) (any, error) {
	return nil, nil
}

type Postgres struct{}

func (p *Postgres) Pool() *PoolHandle { return nil }

func (p *Postgres) Exec(ctx context.Context, sql string, arguments ...any) (any, error) {
	return nil, nil
}

func (p *Postgres) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (p *Postgres) QueryRow(ctx context.Context, sql string, args ...any) any { return nil }

func (p *Postgres) Health(ctx context.Context) error { return nil }

func (p *Postgres) Close() error { return nil }

func (p *Postgres) BeginTransaction(ctx context.Context) (Transaction, error) {
	return nil, nil
}

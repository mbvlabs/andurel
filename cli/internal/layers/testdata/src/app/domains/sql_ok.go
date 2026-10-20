package domains

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// The raw handle is legal inside domains.
func useDB(ctx context.Context, db *storage.Postgres) error {
	_, err := db.Exec(ctx, "select 1")
	return err
}

func keepConnection(conn storage.Connection) storage.Connection {
	return conn
}

type Tokens struct {
	transactor storage.Transactor
}

func (t Tokens) InTx(tx storage.Tx) Tokens {
	return Tokens{transactor: t.transactor.InTx(tx)}
}

// AtomicOps and InTx binding are legal in domains, and bound locals stay clean.
func (t Tokens) Replace(ctx context.Context) error {
	return storage.AtomicOps(ctx, t.transactor, func(ctx context.Context, tx storage.Tx) error {
		tokens := t.InTx(tx)
		return tokens.helper()
	})
}

func (t Tokens) helper() error { return nil }

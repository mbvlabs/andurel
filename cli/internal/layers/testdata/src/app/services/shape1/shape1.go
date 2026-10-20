package shape1

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
	users      domains.Users
}

// Shape 1: a bindable value captured through an alias declared outside the
// callback.
func (p Probe) AliasEscape(ctx context.Context) error {
	users := p.users
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

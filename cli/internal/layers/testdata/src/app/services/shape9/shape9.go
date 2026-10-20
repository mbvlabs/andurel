package shape9

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
	users      domains.Users
}

// Shape 9: the original direct field use remains caught.
func (p Probe) Control(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := p.users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

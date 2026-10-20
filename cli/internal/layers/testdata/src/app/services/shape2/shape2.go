package shape2

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
	users      domains.Users
}

// Shape 2: the callback is a method value. The referenced method body is the
// callback and captures p.users.
func (p Probe) MethodValue(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, p.work)
}

func (p Probe) work(ctx context.Context, tx storage.Tx) error {
	_, err := p.users.Find(ctx, [16]byte{}) // want "tx-escape"
	return err
}

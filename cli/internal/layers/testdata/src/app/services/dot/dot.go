package dot

import (
	"context"

	"app/domains"

	. "github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor Transactor
	users      domains.Users
}

func (p Probe) Dot(ctx context.Context) error {
	users := p.users
	return AtomicOps(ctx, p.transactor, func(ctx context.Context, tx Tx) error {
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

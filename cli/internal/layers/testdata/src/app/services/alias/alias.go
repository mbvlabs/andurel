package alias

import (
	"context"

	"app/domains"

	st "github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor st.Transactor
	users      domains.Users
}

func (p Probe) Alias(ctx context.Context) error {
	users := p.users
	return st.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx st.Tx) error {
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

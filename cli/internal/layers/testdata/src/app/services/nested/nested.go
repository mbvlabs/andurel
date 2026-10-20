package nested

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
	users      domains.Users
}

func each(users domains.Users, fn func(domains.Users) error) error {
	return fn(users)
}

func (p Probe) Bound(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		bound := p.users.InTx(tx)
		return each(bound, func(users domains.Users) error {
			_, err := users.Find(ctx, [16]byte{})
			return err
		})
	})
}

func (p Probe) UnsafeArgument(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		return each(p.users, func(users domains.Users) error { // want "tx-escape"
			_, err := users.Find(ctx, [16]byte{})
			return err
		})
	})
}

func (p Probe) IIFEClean(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		return func(users domains.Users) error {
			_, err := users.Find(ctx, [16]byte{})
			return err
		}(p.users.InTx(tx))
	})
}

func (p Probe) IIFEUnsafe(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		return func(users domains.Users) error {
			_, err := users.Find(ctx, [16]byte{})
			return err
		}(p.users) // want "tx-escape"
	})
}

func (p Probe) OuterCapture(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		return each(p.users.InTx(tx), func(users domains.Users) error {
			_, err := p.users.Find(ctx, [16]byte{}) // want "tx-escape"
			return err
		})
	})
}

func (p Probe) NestedAtomicOps(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		return storage.AtomicOps(ctx, p.transactor.InTx(tx), func(ctx context.Context, inner storage.Tx) error {
			users := p.users.InTx(inner)
			_, err := users.Find(ctx, [16]byte{})
			return err
		})
	})
}

package txwrong

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Service struct {
	transactor storage.Transactor
	users      domains.Users
}

func (s Service) Wrong(ctx context.Context, outer storage.Tx) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := s.users.InTx(outer).Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

func (s Service) Alias(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		current := tx
		users := s.users.InTx(current)
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

type NotAReceiver struct{}

func (NotAReceiver) InTx(tx storage.Tx) domains.Users { return domains.Users{} }

func (s Service) Unrelated(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := NotAReceiver{}.InTx(tx).Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

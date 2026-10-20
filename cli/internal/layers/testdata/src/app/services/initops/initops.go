package initops

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Service struct {
	transactor storage.Transactor
	users      domains.Users
}

var run = func(ctx context.Context, s Service) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := s.users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

package generic

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

var globalUsers domains.Users

type Service struct {
	transactor storage.Transactor
}

func WorkUnsafe[T any](ctx context.Context, tx storage.Tx) error {
	_, err := globalUsers.Find(ctx, [16]byte{}) // want "tx-escape"
	return err
}

func WorkPairUnsafe[A, B any](ctx context.Context, tx storage.Tx) error {
	_, err := globalUsers.Find(ctx, [16]byte{}) // want "tx-escape"
	return err
}

func WorkClean[T any](ctx context.Context, tx storage.Tx) error {
	return nil
}

func (s Service) RunUnsafe(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, WorkUnsafe[int])
}

func (s Service) RunPairUnsafe(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, WorkPairUnsafe[int, string])
}

func (s Service) RunClean(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, WorkClean[int])
}

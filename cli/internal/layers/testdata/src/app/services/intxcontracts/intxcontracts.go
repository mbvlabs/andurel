package intxcontracts

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Service struct {
	transactor storage.Transactor
	pointer    *domains.PointerInTx
	value      domains.ValueInTx
}

// Pointer-receiver InTx returning the value type.
func (s Service) Pointer(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		bound := s.pointer.InTx(tx)
		_, err := bound.Find(ctx, [16]byte{})
		return err
	})
}

// Value-receiver InTx returning the pointer type.
func (s Service) Value(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		bound := s.value.InTx(tx)
		_, err := bound.Find(ctx, [16]byte{})
		return err
	})
}

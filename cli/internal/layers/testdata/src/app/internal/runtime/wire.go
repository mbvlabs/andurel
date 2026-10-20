package runtime

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Wiring struct {
	transactor storage.Transactor
}

// internal/runtime constructs the transactor and may hand it to services.
func (w Wiring) Run(ctx context.Context) error {
	return storage.AtomicOps(ctx, w.transactor, func(ctx context.Context, tx storage.Tx) error {
		return nil
	})
}

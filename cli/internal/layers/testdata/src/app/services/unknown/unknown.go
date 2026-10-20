package unknown

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
}

func (p Probe) Run(
	ctx context.Context,
	callback func(context.Context, storage.Tx) error,
) error {
	return storage.AtomicOps(ctx, p.transactor, callback) // want "tx-escape"
}

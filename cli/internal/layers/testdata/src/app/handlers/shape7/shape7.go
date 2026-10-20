package shape7

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type ProbeHandler struct {
	transactor storage.Transactor // want "tx-layer"
	users      domains.Users
}

// Shape 7: a handler owns the transactor and orchestrates the transaction,
// skipping the service layer.
func (h ProbeHandler) Show(ctx context.Context) error {
	return storage.AtomicOps(ctx, h.transactor, func(ctx context.Context, tx storage.Tx) error { // want "tx-layer"
		_, err := h.users.InTx(tx).Find(ctx, [16]byte{})
		return err
	})
}

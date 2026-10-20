package shape6

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// Shape 6: a free function with no receiver captures an unbound dependency.
func PlainFunc(ctx context.Context, t storage.Transactor, users domains.Users) error {
	return storage.AtomicOps(ctx, t, func(ctx context.Context, tx storage.Tx) error {
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

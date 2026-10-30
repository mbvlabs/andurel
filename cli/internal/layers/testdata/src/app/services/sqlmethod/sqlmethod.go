package sqlmethod

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// Holder keeps a storage handle in a field; the type reference alone is a
// violation outside persistence.
type Holder struct {
	db storage.Connection // want "db-handle"
}

func open() storage.Connection { // want "db-handle"
	return nil
}

// InferredLocal keeps the factory result in an inferred local.
func InferredLocal(ctx context.Context) error {
	db := open()
	_, err := db.Exec(ctx, "delete from users") // want "db-handle"
	return err
}

// ChainedSQL calls SQL on a returned interface value.
func ChainedSQL(ctx context.Context) error {
	_, err := open().Query(ctx, "select 1") // want "db-handle"
	return err
}

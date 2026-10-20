package shape4

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	pg *storage.Postgres
}

// Shape 4: the pool escape hatch exposes a handle that executes SQL.
func (p Probe) Pool(ctx context.Context) error {
	_, err := p.pg.Pool().Exec(ctx, "delete from users") // want "db-handle"
	return err
}

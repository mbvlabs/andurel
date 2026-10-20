package shape3

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	pg *storage.Postgres
}

// Shape 3: raw SQL through the Postgres handle itself.
func (p Probe) RawSQL(ctx context.Context) error {
	_, err := p.pg.Exec(ctx, "delete from users") // want "db-handle"
	return err
}

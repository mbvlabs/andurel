package shape5

import (
	"context"
	"database/sql" // want "db-handle"
)

type Probe struct {
	sqlDB *sql.DB
}

// Shape 5: the standard library SQL handle outside domains.
func (p Probe) StdlibSQL(ctx context.Context) error {
	_, err := p.sqlDB.ExecContext(ctx, "delete from users")
	return err
}

package shape3b

import (
	"context"

	"app/internal/dbfactory"
)

// The service does not import pkg/storage itself; the Postgres handle arrives
// through a factory return value.
func RawSQL(ctx context.Context) error {
	db := dbfactory.Open()
	_, err := db.Exec(ctx, "delete from users") // want "db-handle"
	return err
}

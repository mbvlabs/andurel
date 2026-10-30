package narrowsql

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// embeddedSQL embeds a pkg/storage interface, so its methods are declared by
// pkg/storage even though the interface is local.
type embeddedSQL interface {
	storage.Connection // want "db-handle"
	DeleteAll(ctx context.Context) error
}

// copiedSQL copies the pkg/storage SQL method signatures without embedding
// the interface.
type copiedSQL interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	QueryRow(ctx context.Context, sql string, args ...any) any
}

// notSQL has an Exec method with a different signature; it must not be
// reported as a database handle.
type notSQL interface {
	Exec(name string) error
}

type Probe struct {
	embedded embeddedSQL
	copied   copiedSQL
	other    notSQL
}

func (p Probe) Embedded(ctx context.Context) error {
	_, err := p.embedded.Exec(ctx, "delete from users") // want "db-handle"
	return err
}

func (p Probe) Copied(ctx context.Context) error {
	_ = p.copied.QueryRow(ctx, "select 1") // want "db-handle"
	return nil
}

func (p Probe) NotSQL() error {
	return p.other.Exec("no-op")
}

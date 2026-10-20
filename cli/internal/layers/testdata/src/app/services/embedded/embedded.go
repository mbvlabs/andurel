package embedded

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type DB struct {
	*storage.Postgres
}

type Probe struct {
	db DB
}

func (p Probe) Exec(ctx context.Context) error {
	_, err := p.db.Exec(ctx, "delete from users") // want "db-handle"
	return err
}

func (p Probe) Pool(ctx context.Context) error {
	_, err := p.db.Pool().Exec(ctx, "delete from users") // want "db-handle"
	return err
}

func (p Probe) Query(ctx context.Context) error {
	_, err := p.db.Query(ctx, "select 1") // want "db-handle"
	return err
}

func (p Probe) Begin(ctx context.Context) error {
	_, err := p.db.BeginTransaction(ctx) // want "db-handle"
	return err
}

func (p Probe) Health(ctx context.Context) error {
	return p.db.Health(ctx)
}

func (p Probe) Close() error {
	return p.db.Close()
}

func (p Probe) MethodValue(ctx context.Context) error {
	exec := p.db.Exec // want "db-handle"
	_, err := exec(ctx, "delete from users")
	return err
}

func (p Probe) MethodExpression(ctx context.Context) error {
	exec := (*DB).Exec // want "db-handle"
	_, err := exec(&p.db, ctx, "delete from users")
	return err
}

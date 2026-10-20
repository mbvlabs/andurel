package domains

import (
	"context"

	"app/domains/internal/queries"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Users struct {
	queries *queries.Queries
}

func NewUsers(db storage.Connection) Users {
	return Users{queries: queries.New(db)}
}

func (u Users) InTx(tx storage.Tx) Users {
	return Users{queries: queries.New(storage.Unwrap(tx))}
}

func (u Users) Find(ctx context.Context, id [16]byte) (User, error) {
	return User{}, nil
}

type User struct {
	ID [16]byte
}

type PointerInTx struct{}

func (p *PointerInTx) InTx(tx storage.Tx) PointerInTx { return PointerInTx{} }

func (p PointerInTx) Find(ctx context.Context, id [16]byte) (User, error) {
	return User{}, nil
}

type ValueInTx struct{}

func (v ValueInTx) InTx(tx storage.Tx) *ValueInTx { return &v }

func (v *ValueInTx) Find(ctx context.Context, id [16]byte) (User, error) {
	return User{}, nil
}

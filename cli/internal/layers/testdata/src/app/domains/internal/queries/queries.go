package queries

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Queries struct{}

func New(any) *Queries { return &Queries{} }

func (q *Queries) Find(ctx context.Context, id [16]byte) (pgtype.Text, error) {
	return pgtype.Text{}, nil
}

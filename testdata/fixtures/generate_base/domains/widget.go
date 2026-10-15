package domains

// andurel:table widgets

import (
	"context"
	"errors"
	"time"
	"uuid"

	"example.com/app/domains/internal/queries"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// Widgets is the widget domain's persistence entry point. It owns the widgets
// table; runtime builds it with NewWidgets and services bind it with InTx.
type Widgets struct {
	queries *queries.Queries
}

func NewWidgets(db storage.Connection) Widgets {
	return Widgets{queries: queries.New(db)}
}

// InTx returns a Widgets whose calls run inside tx.
func (w Widgets) InTx(tx storage.Tx) Widgets {
	return Widgets{queries: queries.New(storage.Unwrap(tx))}
}

type Widget struct {
	ID        uuid.UUID          `andurel:"id"`
	Name      string             `andurel:"name"`
	CreatedAt pgtype.Timestamptz `andurel:"created_at"`
	UpdatedAt pgtype.Timestamptz `andurel:"updated_at"`
}

func (w Widgets) Find(ctx context.Context, id uuid.UUID) (Widget, error) {
	entity, err := w.queries.GetWidget[Widget](ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Widget{}, ErrNotFound
		}

		return Widget{}, err
	}

	return entity, nil
}

func (w Widgets) All(ctx context.Context) ([]Widget, error) {
	return w.queries.ListWidgets[Widget](ctx).All()
}

func (w Widgets) Create(ctx context.Context, name string) (Widget, error) {
	return w.queries.CreateWidget[Widget](ctx, queries.CreateWidgetParams{
		ID:        uuid.New(),
		Name:      name,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
}

func (w Widgets) Destroy(ctx context.Context, id uuid.UUID) error {
	return w.queries.DeleteWidget(ctx, id)
}

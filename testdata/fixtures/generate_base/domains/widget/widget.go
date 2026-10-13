package widget

// andurel:table widgets

import (
	"context"
	"errors"
	"time"
	"uuid"

	"example.com/app/domains"
	"example.com/app/domains/internal/queries"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// Widgets is the widget domain's persistence entry point. It owns the widgets
// table and never imports another domain.
type Widgets struct {
	db      storage.Connection
	queries *queries.Queries
}

func NewWidgets(db storage.Connection) Widgets {
	return Widgets{db: db, queries: queries.New(db)}
}

func (w Widgets) WithTx(tx storage.Transaction) Widgets {
	return Widgets{queries: queries.New(tx)}
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
			return Widget{}, domains.ErrNotFound
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

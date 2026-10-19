# Domain

A domain is the complete database and business picture for one entity: its
table, its colocated SQL, its entity with `Validate`, its receiver methods,
and the domain errors it returns. Domains live in one flat `domains` package —
one file per domain, no sub-packages. Only `domains` imports the generated
`domains/internal/queries` package and names `storage.Connection` or unwraps a
`storage.Tx`. Each receiver has an exported `New<Plural>(db)` constructor,
built in `internal/runtime`, and an `InTx(tx)` method that returns the same
receiver bound to an open transaction.

Validate once, at the write path: entities expose `Validate()` via
`pkg/validation`, and `Create`/`Update` call it before touching the database.
Callers (handlers, services) do not repeat entity rules.

The worked example below is a `Product` domain. The reference app's
`domains/user.go` and `domains/token.go` follow the same shape.

## Files

### domains/product.go
```go
package domains

import (
	"context"
	"errors"
	"strings"
	"time"
	"uuid"

	"app/domains/internal/queries"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/pkg/validation"
)

type Products struct {
	queries *queries.Queries
}

func NewProducts(db storage.Connection) Products {
	return Products{queries: queries.New(db)}
}

func (p Products) InTx(tx storage.Tx) Products {
	return Products{queries: queries.New(storage.Unwrap(tx))}
}

type Product struct {
	ID         uuid.UUID          `andurel:"id"`
	CreatedAt  pgtype.Timestamptz `andurel:"created_at"`
	UpdatedAt  pgtype.Timestamptz `andurel:"updated_at"`
	Name       string             `andurel:"name"`
	PriceCents int64              `andurel:"price_cents"`
}

func (p *Product) Validate() error {
	b := validation.NewBuilder()
	b.Required("name", p.Name)
	b.MaxLen("name", p.Name, 255)
	b.MinInt("priceCents", p.PriceCents, 0)

	return b.Err()
}

func (p Products) Find(ctx context.Context, id uuid.UUID) (Product, error) {
	entity, err := p.queries.GetProduct[Product](ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Product{}, ErrNotFound
		}

		return Product{}, err
	}

	return entity, nil
}

type CreateProductData struct {
	Name       string
	PriceCents int64
}

func (p Products) Create(ctx context.Context, data CreateProductData) (Product, error) {
	now := time.Now()
	entity := Product{
		ID:         uuid.New(),
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		Name:       strings.TrimSpace(data.Name),
		PriceCents: data.PriceCents,
	}
	if err := validation.Validate(&entity); err != nil {
		return Product{}, err
	}

	return p.queries.CreateProduct[Product](ctx, queries.CreateProductParams{
		ID:         entity.ID,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
		Name:       entity.Name,
		PriceCents: entity.PriceCents,
	})
}
```

### domains/product.sql
```sql
-- name: GetProduct :one
select * from products where id = $1;

-- name: CreateProduct :one
insert into products (id, created_at, updated_at, name, price_cents)
values ($1, $2, $3, $4, $5)
returning *;

-- name: ListProducts :many
select * from products order by created_at desc;
```

### internal/runtime/runtime.go (edit)
```go
products := domains.NewProducts(db)

handlers.NewProductHandler(products, renderer).Register(router)
```

### Atomic multi-statement methods (when needed)

A receiver whose method runs several statements that must be atomic on their
own holds a `storage.Transactor` and rebinds it in `InTx`. Standalone, the
method opens its own transaction; on a receiver bound with `InTx`, the bound
transactor makes `AtomicOps` join the caller's transaction instead.

```go
// Tokens needs a transactor because ReplaceCode is atomic both standalone and
// when bound to a service's transaction.
type Tokens struct {
	transactor storage.Transactor
	queries    *queries.Queries
}

func NewTokens(db storage.Connection) Tokens {
	return Tokens{transactor: storage.NewTransactor(db), queries: queries.New(db)}
}

func (t Tokens) InTx(tx storage.Tx) Tokens {
	return Tokens{transactor: t.transactor.InTx(tx), queries: queries.New(storage.Unwrap(tx))}
}

func (t Tokens) ReplaceCode(ctx context.Context /* … */) (string, error) {
	var code string
	err := storage.AtomicOps(ctx, t.transactor, func(ctx context.Context, tx storage.Tx) error {
		tokens := t.InTx(tx)
		if err := tokens.DestroyByUserAndScope(ctx /* … */); err != nil {
			return err
		}
		c, err := tokens.CreateCode(ctx /* … */)
		if err != nil {
			return err
		}
		code = c
		return nil
	})
	if err != nil {
		return "", err
	}
	return code, nil
}
```

### domains/product_test.go
```go
package domains_test

import (
	"testing"
	"uuid"

	"app/domains"
	"app/migrations"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestProduct_Validate(t *testing.T) {
	if err := (&domains.Product{}).Validate(); err == nil {
		t.Fatal("expected error for a product without a name")
	}
}

func TestProducts_Create(t *testing.T) {
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	products := domains.NewProducts(db)

	got, err := products.Create(t.Context(), domains.CreateProductData{
		Name:       "Lamp",
		PriceCents: 4500,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID == uuid.Nil() {
		t.Fatal("expected id")
	}
}
```

## Rules

- One file per domain in the flat `domains` package. Cross-entity rules belong in a service.
- Only files under `domains/` may import `domains/internal/queries` or call `storage.Unwrap`; only `domains/` and the wiring layers (`internal/runtime`, `cmd`, `seeds`, `migrations`) may name `storage.Connection`/`storage.Transaction` or call `storage.RunInTransaction`.
- Every receiver has `New<Plural>(db storage.Connection)` and `InTx(tx storage.Tx)`. Construct it in `internal/runtime` and pass it to the services and handlers that use it.
- A method with several statements that must be atomic on its own holds a `storage.Transactor`, rebinds it in `InTx`, and wraps the statements in `storage.AtomicOps` (it joins a caller's transaction when bound).
- Entities expose `Validate()` via `pkg/validation`; `Create`/`Update` normalise, then validate, then write, returning validation errors from `pkg/validation` directly.
- Keep the entity 1:1 with a scanned row.
- SQL lives beside the domain that owns it (`domains/product.sql`); narsilc globs `domains/**/*.sql`.
- Map expected not-found and unique-constraint failures to `ErrNotFound` and `ErrConflict`; return `pkg/validation` errors directly. Preserve unexpected database errors for diagnostics.
- `inspect domains --json` labels table names with `table_source: "inferred"` when derived by convention. For custom names or irregular plurals, add `// andurel:table people` as the entity struct's doc comment (a free-standing annotation only applies when the file has a single entity); inspection reports `table_source: "annotation"`. The annotation is inspection metadata, not SQL configuration.
- Writes that span receivers go through `storage.AtomicOps` in a service (see the `service` pattern).

## Checklist

- [ ] Entity has `Validate()` and `Create`/`Update` reject invalid entities.
- [ ] `<entity>.sql` contains every query the receiver calls.
- [ ] Receiver has `New<Plural>(db)` and `InTx(tx)` and is constructed in `internal/runtime`.
- [ ] Not-found maps to `ErrNotFound`; conflicts to `ErrConflict`.
- [ ] `andurel patterns show domain --entity <Name>` manifest applied in order.
- [ ] `andurel sync queries` succeeds and `andurel verify` passes.

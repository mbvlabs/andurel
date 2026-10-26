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
	"strings"
	"time"
	"uuid"

	"app/domains/internal/queries"

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
	return storage.Row(p.queries.GetProduct[Product](ctx, id))
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
	if err := entity.Validate(); err != nil {
		return Product{}, err
	}

	created, err := p.queries.CreateProduct[Product](ctx, queries.CreateProductParams{
		ID:         entity.ID,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
		Name:       entity.Name,
		PriceCents: entity.PriceCents,
	})

	return storage.Row(created, storage.Conflict(err, "products_name_key"))
}

func (p Products) Paginate(
	ctx context.Context,
	page, pageSize int64,
) (storage.Paginated[Product], error) {
	req := storage.Page(page, pageSize)

	totalCount, err := p.queries.CountProducts(ctx)
	if err != nil {
		return storage.Paginated[Product]{}, err
	}

	entities, err := p.queries.ListProducts[Product](ctx).
		OrderByCreatedAtDesc().
		Limit(req.Limit).
		Offset(req.Offset).
		All()
	if err != nil {
		return storage.Paginated[Product]{}, err
	}

	return storage.NewPaginated(entities, totalCount, req), nil
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
-- @order created_at
select * from products;

-- name: CountProducts :one
select count(*) from products;
```

### internal/runtime/runtime.go (edit)
```go fragment
products := domains.NewProducts(db)

handlers.NewProductHandler(products, renderer).Register(router)
```

### domains/product.go (edit: atomic methods)

A receiver holds a `storage.Transactor` when one of its own methods must be
atomic by itself, either because it writes more than once or because it reads,
modifies and writes a row. Standalone, the method opens its own transaction; on
a receiver bound with `InTx`, the bound transactor makes `AtomicOps` join the
caller's transaction instead.

```go fragment
// A receiver holds a storage.Transactor when one of its own methods must be
// atomic by itself, either because it writes more than once or because it
// reads, modifies and writes a row.
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
	return t.replace(ctx /* … */, func(ctx context.Context, tokens Tokens) (string, error) {
		return tokens.CreateCode(ctx /* … */)
	})
}

func (t Tokens) replace(
	ctx context.Context /* … */,
	create func(context.Context, Tokens) (string, error),
) (string, error) {
	var code string
	err := storage.AtomicOps(ctx, t.transactor, func(ctx context.Context, tx storage.Tx) error {
		tokens := t.InTx(tx)
		if err := tokens.DestroyByUserAndScope(ctx /* … */); err != nil {
			return err
		}
		c, err := create(ctx, tokens)
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

### domains/product.go (edit: read-modify-write)

A read-modify-write method locks its row in the same transaction:

```go fragment
locked, err := users.FindForUpdate(ctx, id)
if err != nil {
	return err
}
locked.Password = newPassword
return users.Save(ctx, locked)
```

## Rules

- One file per domain in the flat `domains` package. Cross-entity rules belong in a service.
- Only files under `domains/` may import `domains/internal/queries` or call `storage.Unwrap`; only `domains/` and the wiring layers (`internal/runtime`, `cmd`, `seeds`, `migrations`) may name `storage.Connection`/`storage.Transaction`.
- Every receiver has `New<Plural>(db storage.Connection)` and `InTx(tx storage.Tx)`. Construct it in `internal/runtime` and pass it to the services and handlers that use it.
- A receiver holds a `storage.Transactor` when one of its methods writes more than once or reads, modifies and writes a row; it rebinds it in `InTx`, wraps the statements in `storage.AtomicOps` (joining a caller's transaction when bound), and locks the row with `FindForUpdate` before a read-modify-write. `Save` writes every column and does not lock for its caller.
- Entities expose `Validate()` via `pkg/validation`; `Create`/`Update` normalise, then validate, then write, returning validation errors from `pkg/validation` directly.
- Keep the entity 1:1 with a scanned row.
- SQL lives beside the domain that owns it (`domains/product.sql`); narsilc globs `domains/**/*.sql`.
- Map expected not-found and unique-constraint failures to `ErrNotFound` and `ErrConflict`; return `pkg/validation` errors directly. Preserve unexpected database errors for diagnostics.
- `inspect domains --json` labels table names with `table_source: "inferred"` when derived by convention. For custom names or irregular plurals, add `// andurel:table people` as the entity struct's doc comment (a free-standing annotation only applies when the file has a single entity); inspection reports `table_source: "annotation"`. The annotation is inspection metadata, not SQL configuration.
- `inspect domains --json` also reports `columns_ok`: true when every column an automatic narsilc query mapping returns has an exported `andurel` tag, false for a missing column or a duplicate exported tag, and null when no query instantiates the entity or `*T` implements `narsilc.Transformer[Row]` and maps in user code. `mismatches` names the column, field, file and line; `type-difference` entries are notes only because pgx may convert between the Row and entity types. `unpopulated_tags` lists extra tagged fields, which narsilc leaves at their zero value. `andurel verify` reports `entity-columns` for missing columns and duplicate tags; fix by adding the exported field/tag or narrowing the query columns and running `andurel sync queries`.
- Writes that span receivers go through `storage.AtomicOps` in a service (see the `service` pattern).
- Test the domain as the `test-domain` pattern shows: `domains/main_test.go` owns the Postgres cluster and `domains/<entity>_test.go` covers `Validate` and the receiver methods.

## Checklist

- [ ] Entity has `Validate()` and `Create`/`Update` reject invalid entities.
- [ ] `<entity>.sql` contains every query the receiver calls.
- [ ] Receiver has `New<Plural>(db)` and `InTx(tx)` and is constructed in `internal/runtime`.
- [ ] Not-found maps to `ErrNotFound`; conflicts to `ErrConflict`.
- [ ] `andurel patterns show domain --entity <Name>` manifest applied in order.
- [ ] `andurel sync queries` succeeds and `andurel verify` passes.

# Test domain

A domain test runs against a real Postgres database with migrations applied.
There are no factories: build entities inline through the domain API. Each
test builds its own receivers over its own clean database, so tests can
run in parallel. `Validate` gets plain unit tests that need no database.

## Files

### domains/product_test.go
```go
package domains_test

import (
	"errors"
	"testing"

	"app/domains"
	"app/migrations"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestProduct_Validate(t *testing.T) {
	cases := []struct {
		name    string
		product domains.Product
		wantErr bool
	}{
		{"valid", domains.Product{Name: "Lamp", PriceCents: 4500}, false},
		{"missing name", domains.Product{PriceCents: 4500}, true},
		{"negative price", domains.Product{Name: "Lamp", PriceCents: -1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.product.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestProducts_Create(t *testing.T) {
	t.Parallel()
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	products := domains.NewProducts(db)

	got, err := products.Create(t.Context(), domains.CreateProductData{
		Name:       "  Lamp ",
		PriceCents: 4500,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Name != "Lamp" {
		t.Fatalf("name = %q, want normalised %q", got.Name, "Lamp")
	}
}

func TestProducts_CreateRejectsInvalid(t *testing.T) {
	t.Parallel()
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	products := domains.NewProducts(db)

	_, err := products.Create(t.Context(), domains.CreateProductData{})
	if !errors.Is(err, domains.ErrViolation) {
		t.Fatalf("err = %v, want ErrViolation", err)
	}
}
```

## Rules

- Domain tests require a real Postgres container; handler tests only when they seed data.
- Unit-test every `Validate` method directly, including the unhappy paths.
- Get a clean database per test with `storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")` and build receivers over it (`domains.NewProducts(db)`). Add `storage.NewTransactor(db)` when the test exercises a service or an atomic method.
- Never share receivers or a database between tests; there are no globals, so per-test construction keeps `t.Parallel()` safe.
- To prove atomicity, make a later step fail (e.g. a queue whose `Insert` errors) and assert the earlier writes were rolled back.
- Build entities inline through the domain API; do not add factories.
- Assert on the entity returned by the domain, not raw SQL rows; match errors with `errors.Is(err, domains.ErrX)`.
- Keep tests in the `_test` package to exercise the public API.

## Checklist

- [ ] `Validate` unit tests cover valid and invalid inputs.
- [ ] One clean database and freshly built receivers per test.
- [ ] No factory package used.
- [ ] Success and error paths both covered.
- [ ] `go test ./domains/...` passes.

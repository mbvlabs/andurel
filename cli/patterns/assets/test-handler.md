# Test handler

A handler test builds a real `httptest` server from actual registrations using
`pkg/anduin/anduintest`. The server itself needs nothing external; a test
needs Postgres only when it seeds data through the domains, as below.

## Files

### handlers/product_test.go
```go
package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"app/domains"
	"app/handlers"
	"app/migrations"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/anduin/anduintest"
	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestProductIndex(t *testing.T) {
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	products := domains.NewProducts(db)
	for i := range 3 {
		_, err := products.Create(t.Context(), domains.CreateProductData{
			Name:       fmt.Sprintf("Product %d", i),
			PriceCents: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	srv := anduintest.New(t, func(rt *anduin.Router) {
		handlers.NewProductHandler(products, testRenderer(t)).Register(rt)
	})

	resp := srv.Get(routes.ProductIndex.Path() + "?page=1")
	resp.AssertStatus(http.StatusOK)
	resp.AssertInertia("Product/Index")
}
```

## Rules

- Use `anduintest.New(t, register)` to build the server from real registrations.
- Provide a local `testRenderer(t)` helper that builds an `*inertia.Renderer`; there is no framework default.
- Call `srv.Get(...)`, `srv.PostForm(...)`, or `srv.PostJSON(...)`; responses assert with `AssertStatus(int)` and `AssertInertia(string)`.
- Prefer handler tests over integration tests when no cross-domain behavior is exercised.
- Assert status and the Inertia component name, not HTML strings.
- Seed data through receivers built per test (`domains.NewProducts(db)`); pass the handler the same receiver.

## Checklist

- [ ] Server built with `anduintest.New`.
- [ ] Response asserted by status and Inertia component.
- [ ] Data seeded through the domain API on per-test receivers.
- [ ] `go test ./handlers/...` passes.

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
)

func TestProductIndex(t *testing.T) {
	db := testCluster.NewTestDB(t, migrations.Migrations, ".")
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

	resp := srv.Get(
		routes.ProductIndex.Path()+"?page=1",
		anduintest.Header{Name: "X-Inertia", Value: "true"},
	)
	resp.AssertStatus(http.StatusOK)
	resp.AssertInertia("Product/Index")
}
```

## Rules

- The scaffold ships `handlers/main_test.go` with the package cluster and `testRenderer(t)`, and `handlers/auth_test.go` as the worked example. Use those files instead of redefining the helpers.
- Use `anduintest.New(t, register)` to build the server from real registrations.
- Do not add another `testRenderer(t)`; `handlers/main_test.go` owns the renderer for the package. It builds the renderer like `runtime.newRenderer`, from the `config.DefaultInertia*` constants, so change renderer settings in `config`, not in the test.
- Call `srv.Get(...)`, `srv.PostForm(...)`, or `srv.PostJSON(...)`; responses assert with `AssertStatus(int)` and `AssertInertia(string)`.
- Send `X-Inertia: true` when asserting a page: `AssertInertia` decodes the Inertia JSON response. Decode `resp.JSON` for props such as `errors`.
- Prefer handler tests over integration tests when no cross-domain behavior is exercised.
- Assert status and the Inertia component name, not HTML strings.
- Seed data through receivers built per test (`domains.NewProducts(db)`); pass the handler the same receiver.
- Tests that seed data share the package cluster from `storagetest.Main` in `handlers/main_test.go` and call `testCluster.NewTestDB(t, migrations.Migrations, ".")` for a clean database.

## Checklist

- [ ] Server built with `anduintest.New`.
- [ ] Response asserted by status and Inertia component.
- [ ] Data seeded through the domain API on per-test receivers.
- [ ] `go test ./handlers/...` passes.

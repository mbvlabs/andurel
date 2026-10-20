# Test handler

A handler test builds a real `httptest` server from actual registrations using
`pkg/anduin/anduintest`. The server itself needs nothing external; a test
needs Postgres only when it seeds data through the domains, as below.

## Files

### handlers/main_test.go
```go
package handlers_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"app/resources"

	"github.com/mbvlabs/andurel/pkg/inertia"
	"github.com/mbvlabs/andurel/pkg/storage/storagetest"
)

// testCluster is one Postgres container for the whole test package.
var testCluster *storagetest.Cluster

func TestMain(m *testing.M) {
	cluster, err := storagetest.NewCluster(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping handler tests: start postgres container: %v\n", err)
		os.Exit(0)
	}
	testCluster = cluster

	code := m.Run()

	_ = testCluster.Close(context.Background())
	os.Exit(code)
}

// testRenderer builds the Inertia renderer handlers expect. There is no
// framework default: the root component is the application's own.
func testRenderer(t *testing.T) *inertia.Renderer {
	t.Helper()

	renderer, err := inertia.NewRenderer(
		inertia.RendererConfig{
			ContainerID:  "app",
			BuildPathURL: "/dist/",
			EntryPoint:   "resources/js/app.ts",
			ViteDevURL:   "http://localhost:5173",
		},
		inertia.WithRoot(resources.Root),
		inertia.WithEnvironment("test"),
	)
	if err != nil {
		t.Fatalf("create renderer: %v", err)
	}

	return renderer
}
```

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
- Tests that seed data share the package cluster from `TestMain` (`handlers/main_test.go`) and call `testCluster.NewTestDB(t, migrations.Migrations, ".")` for a clean database.

## Checklist

- [ ] Server built with `anduintest.New`.
- [ ] Response asserted by status and Inertia component.
- [ ] Data seeded through the domain API on per-test receivers.
- [ ] `go test ./handlers/...` passes.

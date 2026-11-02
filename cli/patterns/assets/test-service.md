# Test service

A service test drives a use case through the service's public method and
checks what it wrote and what it queued. It does not hold the database
handle. Seed data through domain code (`domains.NewProducts(db).Create`), and
record queued jobs with a fake `storage.InsertQueue` instead of reading
`river_job`. The database comes from the package cluster and the
`storagetest` helpers, so the test never touches the development database.

## Files

### services/catalog.go
```go
package services

import (
	"context"
	"fmt"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

// ProductIndexArgs is the job that refreshes search for one product. It holds
// only what the worker needs to reload the product.
type ProductIndexArgs struct {
	Name string `json:"name"`
}

func (ProductIndexArgs) Kind() string { return "product_index" }

// Catalog adds products and queues the index job in one transaction.
type Catalog struct {
	transactor storage.Transactor
	products   domains.Products
	queue      storage.InsertQueue
}

func NewCatalog(
	transactor storage.Transactor,
	products domains.Products,
	queue storage.InsertQueue,
) Catalog {
	return Catalog{transactor: transactor, products: products, queue: queue}
}

func (c Catalog) AddProduct(ctx context.Context, data domains.CreateProductData) (domains.Product, error) {
	var product domains.Product

	err := storage.AtomicOps(ctx, c.transactor, func(ctx context.Context, tx storage.Tx) error {
		products, queue := c.products.InTx(tx), c.queue.InTx(tx)

		created, err := products.Create(ctx, data)
		if err != nil {
			return fmt.Errorf("create product: %w", err)
		}
		if _, err := queue.Insert(ctx, ProductIndexArgs{Name: created.Name}, nil); err != nil {
			return fmt.Errorf("queue product index: %w", err)
		}

		product = created
		return nil
	})
	if err != nil {
		return domains.Product{}, err
	}

	return product, nil
}
```

### services/export_test.go
```go
package services

import (
	"testing"

	"github.com/mbvlabs/andurel/pkg/storage/storagetest"
)

// TestCluster is the Postgres cluster for this folder's tests. TestMain lives
// in this package so one cluster serves both package services and package
// services_test in this folder.
var TestCluster *storagetest.Cluster

func TestMain(m *testing.M) {
	storagetest.Main(m, &TestCluster)
}
```

### services/catalog_test.go
```go
package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"app/domains"
	"app/migrations"
	"app/services"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// recordingQueue stands in for the River queue. It records each job's args
// instead of writing river_job, so the test asserts what the service queued.
// It has no transaction, so InTx returns the same recorder: jobs are recorded
// when Insert is called. Commit ordering is covered by the real queue, not here.
type recordingQueue struct {
	mu   sync.Mutex
	jobs []river.JobArgs
}

func (q *recordingQueue) Insert(
	_ context.Context,
	args river.JobArgs,
	_ *river.InsertOpts,
) (*rivertype.JobInsertResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, args)
	return &rivertype.JobInsertResult{}, nil
}

func (q *recordingQueue) InsertMany(
	context.Context,
	[]river.InsertManyParams,
) ([]*rivertype.JobInsertResult, error) {
	return nil, errors.New("recordingQueue: InsertMany is not used")
}

func (q *recordingQueue) InsertManyFast(context.Context, []river.InsertManyParams) (int, error) {
	return 0, errors.New("recordingQueue: InsertManyFast is not used")
}

func (q *recordingQueue) InTx(storage.Tx) storage.InsertQueue { return q }

func TestCatalogAddProductQueuesIndexJob(t *testing.T) {
	t.Parallel()
	db := services.TestCluster.NewTestDB(t, migrations.Migrations, ".")
	queue := &recordingQueue{}
	catalog := services.NewCatalog(storage.NewTransactor(db), domains.NewProducts(db), queue)

	product, err := catalog.AddProduct(t.Context(), domains.CreateProductData{
		Name:       "Lamp",
		PriceCents: 4500,
	})
	if err != nil {
		t.Fatalf("add product: %v", err)
	}

	if len(queue.jobs) != 1 {
		t.Fatalf("queued %d jobs, want 1", len(queue.jobs))
	}
	job, ok := queue.jobs[0].(services.ProductIndexArgs)
	if !ok {
		t.Fatalf("job = %T, want services.ProductIndexArgs", queue.jobs[0])
	}
	if job.Name != product.Name {
		t.Fatalf("job name = %q, want %q", job.Name, product.Name)
	}
}

func TestCatalogAddProductRejectsInvalidWithoutQueueing(t *testing.T) {
	t.Parallel()
	db := services.TestCluster.NewTestDB(t, migrations.Migrations, ".")
	products := domains.NewProducts(db)

	// Seed through the domain, not raw SQL: an existing product is in the
	// database before the service runs.
	if _, err := products.Create(t.Context(), domains.CreateProductData{
		Name:       "Desk",
		PriceCents: 12000,
	}); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	queue := &recordingQueue{}
	catalog := services.NewCatalog(storage.NewTransactor(db), products, queue)

	_, err := catalog.AddProduct(t.Context(), domains.CreateProductData{})
	if err == nil {
		t.Fatal("add product = nil error, want a validation error")
	}
	if len(queue.jobs) != 0 {
		t.Fatalf("queued %d jobs for a rejected product, want 0", len(queue.jobs))
	}
}
```

## Rules

- Build the database with `services.TestCluster.NewTestDB(t, migrations.Migrations, ".")`. Use `storagetest` helpers, never `pgx` or `storage.Connection` directly in a test. Read `andurel patterns show test-domain` for the cluster and per-test database.
- Build the service from `storage.NewTransactor(db)` and the domain receivers over the same `db`. Pass a fake `storage.InsertQueue` (a recorder) for jobs. Assert on `queue.jobs`, and assert the concrete args type.
- Seed data through domain code (`domains.NewProducts(db).Create(...)`). Do not insert rows with SQL, and do not read `river_job`.
- To prove a multi-write use case rolls back, wrap the transactor with `storagetest.FailingTransactor(db, storagetest.StatementFault{...})`. The failing statement makes `storage.AtomicOps` roll back the earlier writes, so assert those writes are gone.
- Assert with `errors.Is` against domain or service sentinels, and `validation.As` for input errors.
- Keep one `TestMain` per folder. In a folder with both `package services` and `package services_test` files, the cluster is declared once in an `export_test.go` (`package services`) and the external package uses the exported `services.TestCluster`. Both packages compile into the same test binary, so they share one cluster and one container. An external package cannot see identifiers declared in another package's test files, so the export is one way.
- Call `t.Parallel()` only when each test builds its own database and receivers. The recorder is safe for concurrent use.
- Keep fakes local to the test file. Do not add a shared fake package.

## Checklist

- [ ] Service built from `storage.NewTransactor(db)`, domain receivers, and a recording queue.
- [ ] Queued jobs asserted by concrete args type and content.
- [ ] Seed data written through domain code, not SQL.
- [ ] Rejected input asserted to queue nothing.
- [ ] No `storage.Connection` or raw SQL in the test file.
- [ ] `go test ./services/...` passes with Postgres available.

package storagetest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/testcontainers/testcontainers-go"
)

// TestClusterNewTestDB needs Docker. It skips cleanly when a container cannot
// be started, so the storage module's unit tests stay Docker-independent.
func TestClusterNewTestDB(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()
	migrations := widgetMigrations()

	first := cluster.NewTestDB(t, migrations, "migrations")
	if _, err := first.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "lamp"); err != nil {
		t.Fatalf("insert into first test database: %v", err)
	}
	assertWidgetCount(t, first, 1)

	second := cluster.NewTestDB(t, migrations, "migrations")
	assertWidgetCount(t, second, 0)

	assertWidgetCount(t, first, 1)
}

// TestClusterNewTestDBWithConfig proves the returned config reaches the same
// migrated database as the returned connection.
func TestClusterNewTestDBWithConfig(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()

	db, config := cluster.NewTestDBWithConfig(t, widgetMigrations(), "migrations")
	if _, err := db.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "lamp"); err != nil {
		t.Fatalf("insert into configured test database: %v", err)
	}

	additional, err := storage.NewPostgres(ctx, config)
	if err != nil {
		t.Fatalf("connect with returned config: %v", err)
	}
	// Registering after NewTestDBWithConfig makes this close before its
	// database-drop cleanup runs.
	t.Cleanup(func() {
		_ = additional.Close()
	})

	var name string
	if err := additional.QueryRow(ctx, `SELECT name FROM widgets`).Scan(&name); err != nil {
		t.Fatalf("read seeded row through configured connection: %v", err)
	}
	if name != "lamp" {
		t.Fatalf("seeded name = %q, want %q", name, "lamp")
	}

	if _, err := additional.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "desk"); err != nil {
		t.Fatalf("insert through configured connection: %v", err)
	}
	assertWidgetCount(t, db, 2)
}

func TestClusterNewTestDBAtVersionMigratesOnward(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()
	migrations := versionedWidgetMigrations()

	db := cluster.NewTestDBAtVersion(t, migrations, "migrations", 1)
	assertWidgetColumn(t, db, "color", false)

	MigrateTestDB(t, db, migrations, "migrations", 2)
	assertWidgetColumn(t, db, "color", true)

	if _, err := db.Exec(
		ctx,
		`INSERT INTO widgets (name, color) VALUES ($1, $2)`,
		"lamp",
		"red",
	); err != nil {
		t.Fatalf("insert after migrating on: %v", err)
	}
}

func TestMigrateTestDBToHead(t *testing.T) {
	cluster := newTestCluster(t)
	migrations := versionedWidgetMigrations()

	db := cluster.NewTestDBAtVersion(t, migrations, "migrations", 1)
	assertWidgetColumn(t, db, "color", false)

	MigrateTestDB(t, db, migrations, "migrations", MigrationHead)
	assertWidgetColumn(t, db, "color", true)
}

func TestAtomicOpsReadOnlyRejectsWrites(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()
	db := cluster.NewTestDB(t, widgetMigrations(), "migrations")

	err := storage.AtomicOps(ctx, storage.NewTransactor(db), func(ctx context.Context, tx storage.Tx) error {
		_, err := storage.Unwrap(tx).Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "lamp")

		return err
	}, storage.ReadOnly())
	if err == nil {
		t.Fatal("read-only transaction accepted a write")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Fatalf("write error = %v, want SQLSTATE 25006", err)
	}
	assertWidgetCount(t, db, 0)
}

func TestAtomicOpsAppliesIsolationToPostgres(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()
	db := cluster.NewTestDB(t, widgetMigrations(), "migrations")

	var level string
	err := storage.AtomicOps(ctx, storage.NewTransactor(db), func(ctx context.Context, tx storage.Tx) error {
		return storage.Unwrap(tx).
			QueryRow(ctx, `SHOW transaction_isolation`).
			Scan(&level)
	}, storage.Isolation(storage.RepeatableRead))
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if level != "repeatable read" {
		t.Fatalf("transaction isolation = %q, want %q", level, "repeatable read")
	}
}

func TestFailingTransactorRollsBackNthStatement(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()
	db := cluster.NewTestDB(t, widgetMigrations(), "migrations")
	transactor := FailingTransactor(db, StatementFault{FailAt: 2})

	err := storage.AtomicOps(ctx, transactor, func(ctx context.Context, tx storage.Tx) error {
		statements := storage.Unwrap(tx)
		if _, err := statements.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "lamp"); err != nil {
			return err
		}

		_, err := statements.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "desk")

		return err
	})
	if !errors.Is(err, ErrFaultInjected) {
		t.Fatalf("AtomicOps error = %v, want %v", err, ErrFaultInjected)
	}
	assertWidgetCount(t, db, 0)
}

func newTestCluster(t *testing.T) *Cluster {
	t.Helper()

	if testing.Short() {
		t.Skip("storagetest: Postgres container test skipped in -short mode")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)

	cluster, err := NewCluster(context.Background())
	if err != nil {
		t.Skipf("storagetest: Postgres container unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = cluster.Close(context.Background())
	})
	return cluster
}

func widgetMigrations() fstest.MapFS {
	return fstest.MapFS{
		"migrations/00001_create_widgets.sql": &fstest.MapFile{
			Data: []byte(`-- +goose Up
CREATE TABLE widgets (
	id bigint PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	name text NOT NULL
);

-- +goose Down
DROP TABLE widgets;
`),
		},
	}
}

func versionedWidgetMigrations() fstest.MapFS {
	return fstest.MapFS{
		"migrations/00001_create_widgets.sql": &fstest.MapFile{
			Data: []byte(`-- +goose Up
CREATE TABLE widgets (
	id bigint PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	name text NOT NULL
);

-- +goose Down
DROP TABLE widgets;
`),
		},
		"migrations/00002_add_widget_color.sql": &fstest.MapFile{
			Data: []byte(`-- +goose Up
ALTER TABLE widgets ADD COLUMN color text;

-- +goose Down
ALTER TABLE widgets DROP COLUMN color;
`),
		},
	}
}

func assertWidgetColumn(t *testing.T, db storage.Connection, column string, want bool) {
	t.Helper()

	var got bool
	if err := db.QueryRow(
		context.Background(),
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'widgets'
			  AND column_name = $1
		)`,
		column,
	).Scan(&got); err != nil {
		t.Fatalf("check widgets.%s: %v", column, err)
	}
	if got != want {
		t.Fatalf("widgets.%s exists = %v, want %v", column, got, want)
	}
}

func assertWidgetCount(t *testing.T, db storage.Connection, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM widgets`).Scan(&got); err != nil {
		t.Fatalf("count widgets: %v", err)
	}
	if got != want {
		t.Fatalf("widget count = %d, want %d", got, want)
	}
}

type faultedJobArgs struct{}

func (faultedJobArgs) Kind() string { return "faulted_job" }

// TestFailingTransactorKeepsTransactionalQueueInsertAndRollsBackBoth proves
// the injected transactor still exposes the concrete pgx transaction to
// QueueInsert.InTx. A domain write and a River job insert succeed, then the
// second matched statement fails, and AtomicOps rolls back both.
func TestFailingTransactorKeepsTransactionalQueueInsertAndRollsBackBoth(t *testing.T) {
	cluster := newTestCluster(t)
	ctx := context.Background()
	db, config := cluster.NewTestDBWithConfig(t, widgetMigrations(), "migrations")

	databaseURL, err := config.DatabaseURL()
	if err != nil {
		t.Fatalf("database URL: %v", err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open migrator pool: %v", err)
	}
	t.Cleanup(pool.Close)

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatalf("new river migrator: %v", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatalf("migrate river schema: %v", err)
	}

	queue, err := storage.NewQueueInsert(db, storage.DefaultQueueConfig())
	if err != nil {
		t.Fatalf("new queue insert: %v", err)
	}

	transactor := FailingTransactor(db, StatementFault{
		FailAt: 2,
		Match: func(query string) bool {
			return strings.Contains(query, "INSERT INTO widgets")
		},
	})

	var jobID int64
	err = storage.AtomicOps(ctx, transactor, func(ctx context.Context, tx storage.Tx) error {
		if _, err := storage.Unwrap(tx).Exec(
			ctx,
			`INSERT INTO widgets (name) VALUES ($1)`,
			"before queue",
		); err != nil {
			return err
		}

		inserted, err := queue.InTx(tx).Insert(ctx, faultedJobArgs{}, nil)
		if err != nil {
			return err
		}
		jobID = inserted.Job.ID

		_, err = storage.Unwrap(tx).Exec(
			ctx,
			`INSERT INTO widgets (name) VALUES ($1)`,
			"after queue",
		)
		return err
	})
	if !errors.Is(err, ErrFaultInjected) {
		t.Fatalf("AtomicOps error = %v, want %v", err, ErrFaultInjected)
	}
	if jobID == 0 {
		t.Fatal("queue insert returned no job ID; the transaction was not used")
	}

	assertWidgetCount(t, db, 0)

	var jobs int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM river_job`).Scan(&jobs); err != nil {
		t.Fatalf("count river jobs: %v", err)
	}
	if jobs != 0 {
		t.Fatalf("river job count = %d, want 0 after rollback", jobs)
	}
}

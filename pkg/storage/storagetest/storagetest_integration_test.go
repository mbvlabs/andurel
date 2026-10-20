package storagetest

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/mbvlabs/andurel/pkg/storage"
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

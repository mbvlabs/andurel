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
	if testing.Short() {
		t.Skip("storagetest: Postgres container test skipped in -short mode")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx := context.Background()
	cluster, err := NewCluster(ctx)
	if err != nil {
		t.Skipf("storagetest: Postgres container unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = cluster.Close(context.Background())
	})

	migrations := fstest.MapFS{
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

	first := cluster.NewTestDB(t, migrations, "migrations")
	if _, err := first.Exec(ctx, `INSERT INTO widgets (name) VALUES ($1)`, "lamp"); err != nil {
		t.Fatalf("insert into first test database: %v", err)
	}
	assertWidgetCount(t, first, 1)

	second := cluster.NewTestDB(t, migrations, "migrations")
	assertWidgetCount(t, second, 0)

	assertWidgetCount(t, first, 1)
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

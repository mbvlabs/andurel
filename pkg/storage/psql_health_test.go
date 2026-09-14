package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresHealth(t *testing.T) {
	t.Run("unconfigured", func(t *testing.T) {
		db := &Postgres{}
		err := db.Health(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "storage: ping database") {
			t.Fatalf("Health error = %q, want storage context", err)
		}
	})

	t.Run("closed pool", func(t *testing.T) {
		cfg, err := pgxpool.ParseConfig("postgres://storage:storage@127.0.0.1:1/storage")
		if err != nil {
			t.Fatalf("ParseConfig: %v", err)
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			t.Fatalf("NewWithConfig: %v", err)
		}
		pool.Close()

		db := &Postgres{pool: pool}
		err = db.Health(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "storage: ping database") {
			t.Fatalf("Health error = %q, want storage context", err)
		}
		if !strings.Contains(err.Error(), "closed") {
			t.Fatalf("Health error = %v, want closed pool", err)
		}
	})
}

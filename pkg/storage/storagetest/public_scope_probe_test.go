package storagetest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mbvlabs/andurel/pkg/storage/storagetest"
)

// TestPublicScopeProbe is an opt-in helper for the isolated shared-daemon
// startup probe. It uses only the public API and coordinates through a bind
// mounted directory. Never enable it against an unisolated owner hostname.
func TestPublicScopeProbe(t *testing.T) {
	role, dir := os.Getenv("STORAGETEST_PUBLIC_PROBE_ROLE"), os.Getenv("STORAGETEST_PUBLIC_PROBE_DIR")
	if role == "" || dir == "" {
		t.Skip("isolated public startup probe only")
	}
	ctx := context.Background()
	cluster, err := storagetest.NewCluster(ctx)
	if err != nil {
		t.Fatalf("NewCluster: %v", err)
	}
	t.Cleanup(func() {
		if err := cluster.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	db := cluster.NewTestDB(t, fstest.MapFS{"migrations/00001_probe.sql": {Data: []byte("-- +goose Up\nCREATE TABLE probe (value text NOT NULL);\n-- +goose Down\nDROP TABLE probe;\n")}}, "migrations")
	if _, err := db.Exec(ctx, "INSERT INTO probe VALUES ($1)", role); err != nil {
		t.Fatal(err)
	}
	check := func(phase string) {
		t.Helper()
		var value string
		if err := db.QueryRow(ctx, "SELECT value FROM probe").Scan(&value); err != nil || value != role {
			t.Fatalf("%s SQL = %q, %v; want %q", phase, value, err, role)
		}
		fmt.Printf("%s SQL %s OK\n", role, phase)
	}
	check("before")
	if err := os.WriteFile(filepath.Join(dir, role+".ready"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Minute)
	for {
		if _, err := os.Stat(filepath.Join(dir, role+".continue")); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("probe coordination timed out")
		}
		time.Sleep(100 * time.Millisecond)
	}
	check("after")
}

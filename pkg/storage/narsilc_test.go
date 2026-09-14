package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasQueryFiles(t *testing.T) {
	root := t.TempDir()

	hasQueries, err := HasQueryFiles(root)
	if err != nil {
		t.Fatalf("HasQueryFiles empty project: %v", err)
	}
	if hasQueries {
		t.Fatal("expected no query files in empty project")
	}

	queriesDir := filepath.Join(root, QueriesDir)
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatalf("mkdir queries: %v", err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatalf("write gitkeep: %v", err)
	}

	hasQueries, err = HasQueryFiles(root)
	if err != nil {
		t.Fatalf("HasQueryFiles gitkeep only: %v", err)
	}
	if hasQueries {
		t.Fatal("expected .gitkeep not to count as a query file")
	}

	if err := os.WriteFile(filepath.Join(queriesDir, "users.sql"), []byte("-- name: ListUsers :many\nSELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write query file: %v", err)
	}

	hasQueries, err = HasQueryFiles(root)
	if err != nil {
		t.Fatalf("HasQueryFiles with sql: %v", err)
	}
	if !hasQueries {
		t.Fatal("expected query file to be detected")
	}

	if err := os.WriteFile(
		filepath.Join(queriesDir, "stub.sql"),
		[]byte("-- Example:\n-- -- name: ListItems :many\n"),
		0o644,
	); err != nil {
		t.Fatalf("write stub query file: %v", err)
	}

	hasQueries, err = HasQueryFiles(root)
	if err != nil {
		t.Fatalf("HasQueryFiles commented stub: %v", err)
	}
	if !hasQueries {
		t.Fatal("expected active query file to still count after adding a commented stub")
	}

	if err := os.WriteFile(filepath.Join(queriesDir, "users.sql"), []byte("-- Example:\n-- -- name: ListUsers :many\n"), 0o644); err != nil {
		t.Fatalf("overwrite query file: %v", err)
	}

	hasQueries, err = HasQueryFiles(root)
	if err != nil {
		t.Fatalf("HasQueryFiles only commented stubs: %v", err)
	}
	if hasQueries {
		t.Fatal("expected commented stubs not to count as narsilc queries")
	}
}

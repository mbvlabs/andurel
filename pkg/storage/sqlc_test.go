package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLCConfigContainsExpectedPaths(t *testing.T) {
	config := string(SQLCConfig())
	for _, want := range []string{
		"models/queries",
		"models/internal/queries",
		`sql_package: "database/sql"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("sqlc config missing %q\n%s", want, config)
		}
	}
}

func TestWriteSQLCConfig(t *testing.T) {
	root := t.TempDir()

	if err := WriteSQLCConfig(root); err != nil {
		t.Fatalf("WriteSQLCConfig: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(root, SQLCConfigFile))
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if string(written) != string(SQLCConfig()) {
		t.Fatal("written sqlc config does not match embedded config")
	}
}

func TestHasSQLCQueryFiles(t *testing.T) {
	root := t.TempDir()

	hasQueries, err := HasSQLCQueryFiles(root)
	if err != nil {
		t.Fatalf("HasSQLCQueryFiles empty project: %v", err)
	}
	if hasQueries {
		t.Fatal("expected no query files in empty project")
	}

	queriesDir := filepath.Join(root, SQLCQueriesDir)
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		t.Fatalf("mkdir queries: %v", err)
	}
	if err := os.WriteFile(filepath.Join(queriesDir, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatalf("write gitkeep: %v", err)
	}

	hasQueries, err = HasSQLCQueryFiles(root)
	if err != nil {
		t.Fatalf("HasSQLCQueryFiles gitkeep only: %v", err)
	}
	if hasQueries {
		t.Fatal("expected .gitkeep not to count as a query file")
	}

	if err := os.WriteFile(filepath.Join(queriesDir, "users.sql"), []byte("-- name: ListUsers :many\nSELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write query file: %v", err)
	}

	hasQueries, err = HasSQLCQueryFiles(root)
	if err != nil {
		t.Fatalf("HasSQLCQueryFiles with sql: %v", err)
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

	hasQueries, err = HasSQLCQueryFiles(root)
	if err != nil {
		t.Fatalf("HasSQLCQueryFiles commented stub: %v", err)
	}
	if hasQueries {
		t.Fatal("expected commented stub not to count as sqlc queries")
	}
}

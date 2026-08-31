package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestGenerateSQLCQueryFromTemplate(t *testing.T) {
	root := t.TempDir()
	queryPath := filepath.Join(root, storage.SQLCQueriesDir, "user_report.sql")

	if err := generateSQLCQueryFromTemplate(queryPath, sqlcQueryTemplateData{
		PascalName: "UserReport",
	}); err != nil {
		t.Fatalf("generate sqlc query: %v", err)
	}

	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("read query file: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"UserReport",
		"andurel generate queries",
		"-- Example:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in commented template:\n%s", want, text)
		}
	}
	if strings.Contains(text, "-- name: ListUserReport") {
		t.Fatal("commented template should not include active sqlc query")
	}
}

func TestGenerateSQLCQueryFromTemplateWithTable(t *testing.T) {
	root := t.TempDir()
	queryPath := filepath.Join(root, storage.SQLCQueriesDir, "user_report.sql")

	if err := generateSQLCQueryFromTemplate(queryPath, sqlcQueryTemplateData{
		PascalName: "UserReport",
		TableName:  "users",
	}); err != nil {
		t.Fatalf("generate sqlc query with table: %v", err)
	}

	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("read query file: %v", err)
	}
	text := string(content)
	for _, want := range []string{"-- name: ListUserReport :many", "FROM users"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in table template:\n%s", want, text)
		}
	}
}

func TestGenerateSQLCQueryFromTemplateRejectsExistingFile(t *testing.T) {
	root := t.TempDir()
	queryPath := filepath.Join(root, storage.SQLCQueriesDir, "user_report.sql")
	if err := os.MkdirAll(filepath.Dir(queryPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(queryPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := generateSQLCQueryFromTemplate(queryPath, sqlcQueryTemplateData{
		PascalName: "UserReport",
	}); err == nil {
		t.Fatal("expected error for existing file")
	}
}

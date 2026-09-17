package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestGenerateQueryFromTemplate(t *testing.T) {
	root := t.TempDir()
	queryPath := filepath.Join(root, storage.QueriesDir, "user_report.sql")

	if err := generateQueryFromTemplate(queryPath, queryTemplateData{
		PascalName: "UserReport",
	}); err != nil {
		t.Fatalf("generate narsilc query: %v", err)
	}

	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("read query file: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"UserReport",
		"andurel sync queries",
		"-- Example:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in commented template:\n%s", want, text)
		}
	}
	if containsUncommentedQuery(text, "ListUserReport") {
		t.Fatal("commented template should not include active narsilc query")
	}
}

func containsUncommentedQuery(text, queryName string) bool {
	marker := "-- name: " + queryName
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, marker) && !strings.HasPrefix(trimmed, "-- -- name:") {
			return true
		}
	}
	return false
}

func TestGenerateQueryFromTemplateWithTable(t *testing.T) {
	root := t.TempDir()
	queryPath := filepath.Join(root, storage.QueriesDir, "user_report.sql")

	if err := generateQueryFromTemplate(queryPath, queryTemplateData{
		PascalName: "UserReport",
		TableName:  "users",
	}); err != nil {
		t.Fatalf("generate narsilc query with table: %v", err)
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

func TestGenerateQueryFromTemplateRejectsExistingFile(t *testing.T) {
	root := t.TempDir()
	queryPath := filepath.Join(root, storage.QueriesDir, "user_report.sql")
	if err := os.MkdirAll(filepath.Dir(queryPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(queryPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := generateQueryFromTemplate(queryPath, queryTemplateData{
		PascalName: "UserReport",
	}); err == nil {
		t.Fatal("expected error for existing file")
	}
}

func TestGenerateQueryFileRejectsUnsafeNamesAndTables(t *testing.T) {
	for _, test := range []struct {
		name  string
		table string
	}{
		{name: "../escape"},
		{name: "userReport"},
		{name: "UserReport", table: "users; DROP TABLE users"},
	} {
		if err := generateQueryFile(test.name, test.table); err == nil {
			t.Fatalf("generateQueryFile(%q, %q) should reject unsafe input", test.name, test.table)
		}
	}
}

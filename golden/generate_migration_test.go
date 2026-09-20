package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

const dummyMigrationBody = `-- +goose Up
-- +goose StatementBegin
CREATE TABLE widgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE widgets;
-- +goose StatementEnd
`

const dummyProjectEnv = `DB_KIND=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=andurel
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSL_MODE=disable
`

func TestGenerateMigration(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")
	goldentest.WriteProjectFile(t, project, ".env", dummyProjectEnv)
	if err := os.MkdirAll(filepath.Join(project, "migrations"), 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}

	goldentest.RunCLI(t, project, "generate", "migration", "create_widgets")

	rel, err := findGeneratedMigration(project, "create_widgets")
	if err != nil {
		t.Fatal(err)
	}
	// goose create emits empty Up/Down stubs. Seed a dummy CREATE TABLE so the
	// golden locks a representative migration body plus the CLI filename pattern.
	goldentest.WriteProjectFile(t, project, rel, dummyMigrationBody)

	g := goldentest.NewGoldie(t)
	goldentest.AssertFile(
		t,
		g,
		"generate/migration/create_widgets/migrations/create_widgets.sql",
		project,
		rel,
	)
}

func findGeneratedMigration(projectDir, name string) (string, error) {
	suffix := "_" + name + ".sql"
	entries, err := os.ReadDir(filepath.Join(projectDir, "migrations"))
	if err != nil {
		return "", err
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			matches = append(matches, filepath.ToSlash(filepath.Join("migrations", entry.Name())))
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("expected exactly one migrations/*_%s.sql, found %v", name, matches)
	}
	return matches[0], nil
}

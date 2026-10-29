package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

var generatedMigrationName = regexp.MustCompile(`^migrations/\d{14}_[a-z0-9_]+\.sql$`)

func TestGenerateMigration(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")

	// Migration creation writes a SQL skeleton and must not need .env, a
	// configured database, or bin/goose.
	goldentest.RunCLI(t, project, "generate", "migration", "create_widgets")

	rel, err := findGeneratedMigration(project, "create_widgets")
	if err != nil {
		t.Fatal(err)
	}
	if !generatedMigrationName.MatchString(rel) {
		t.Fatalf("migration path %q does not match goose timestamp pattern", rel)
	}

	g := goldentest.NewGoldie(t)
	goldentest.AssertFile(
		t,
		g,
		"generate/migration/create_widgets/migrations/create_widgets.sql",
		project,
		rel,
	)
}

func TestGenerateMigrationDoesNotNeedDatabaseConfiguration(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")
	envPath := filepath.Join(project, ".env")
	if _, err := os.Stat(envPath); err == nil {
		t.Fatalf("fixture unexpectedly ships .env")
	}

	goldentest.RunCLI(t, project, "generate", "migration", "create_widgets")
	if _, err := findGeneratedMigration(project, "create_widgets"); err != nil {
		t.Fatal(err)
	}
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

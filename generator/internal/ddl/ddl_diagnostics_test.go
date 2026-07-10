package ddl

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestDDLDiagnosticFixturesAreDeterministicAndSafe(t *testing.T) {
	parser := NewDDLParser()
	for _, test := range []struct {
		name      string
		directory string
		wantError string
	}{
		{name: "supported", directory: "supported_schema_changes"},
		{name: "unsupported", directory: "unsupported_schema_changes", wantError: "unsupported schema-changing DDL"},
		{name: "ambiguous", directory: "ambiguous_schema_changes", wantError: "unsupported schema-changing DDL"},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths, err := filepath.Glob(filepath.Join("testdata", "ddl_diagnostics", test.directory, "*.sql"))
			if err != nil {
				t.Fatalf("glob fixtures: %v", err)
			}
			if len(paths) == 0 {
				t.Fatalf("no fixtures found in %s", test.directory)
			}
			for _, path := range paths {
				sql, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				_, firstErr := parser.Parse(string(sql), filepath.Base(path), "postgresql")
				_, secondErr := parser.Parse(string(sql), filepath.Base(path), "postgresql")
				if test.wantError == "" {
					if firstErr != nil {
						t.Fatalf("supported fixture %s: %v", path, firstErr)
					}
					continue
				}
				if firstErr == nil || !strings.Contains(firstErr.Error(), test.wantError) {
					t.Fatalf("fixture %s error = %v", path, firstErr)
				}
				if secondErr == nil || secondErr.Error() != firstErr.Error() {
					t.Fatalf("fixture %s diagnostic is not deterministic: %v then %v", path, firstErr, secondErr)
				}
			}
		})
	}
}

func TestApplyDDLRejectsModelAffectingUnsupportedStatements(t *testing.T) {
	cat := catalog.NewCatalog("public")
	for _, sql := range []string{
		"CREATE TABLE archived_users AS SELECT * FROM users",
		"CREATE VIEW active_users AS SELECT * FROM users",
		"ALTER TABLE users ENABLE ROW LEVEL SECURITY",
		"DROP TABLE users CASCADE",
		"DROP SCHEMA public CASCADE",
		"DROP TYPE status CASCADE",
		"DO $$ BEGIN EXECUTE 'ALTER TABLE users ADD COLUMN unsafe text'; END $$",
	} {
		err := ApplyDDL(cat, sql, "002_unsupported.sql", "postgresql")
		if err == nil {
			t.Fatalf("expected %q to fail", sql)
		}
		var unsupported *UnsupportedStatementError
		if !errors.As(err, &unsupported) {
			t.Fatalf("expected UnsupportedStatementError for %q, got %T %v", sql, err, err)
		}
		if !strings.Contains(err.Error(), "split the migration") {
			t.Fatalf("error is not actionable: %v", err)
		}
	}
}

func TestApplyDDLWarnsOnlyForModelNeutralUnsupportedStatements(t *testing.T) {
	var logOutput bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	cat := catalog.NewCatalog("public")
	for _, sql := range []string{
		"VACUUM users",
		"COMMENT ON TABLE users IS 'application data'",
		"INSERT INTO users (id) VALUES (1)",
	} {
		if err := ApplyDDL(cat, sql, "003_harmless.sql", "postgresql"); err != nil {
			t.Fatalf("model-neutral statement %q: %v", sql, err)
		}
	}
	if count := strings.Count(logOutput.String(), "Unknown DDL statement type"); count != 3 {
		t.Fatalf("warning count = %d, want 3:\n%s", count, logOutput.String())
	}
}

func TestDDLParserRejectsMissingMalformedDuplicatedAndAmbiguousStructures(t *testing.T) {
	for _, sql := range []string{
		"CREATE TABLE",
		"CREATE TABLE users ()",
		"CREATE TABLE users (id UUID, id TEXT)",
		"CREATE TABLE users (id UUID, PRIMARY KEY ())",
		"CREATE TABLE users (id UUID, PRIMARY KEY (id), PRIMARY KEY (id))",
		"CREATE TABLE users (id UUID, FOREIGN KEY (id) REFERENCES accounts)",
		"CREATE TABLE users (id UUID",
		"CREATE TABLE users (\"id UUID)",
		"CREATE TABLE users (id UUID); DROP TABLE accounts",
		"CREATE TABLE \"users\" (id UUID)",
		"ALTER TABLE users",
		"ALTER TABLE users ADD COLUMN",
		"ALTER TABLE users ALTER COLUMN email SET STATISTICS 100",
	} {
		_, err := NewDDLParser().Parse(sql, "004_ambiguous.sql", "postgresql")
		if err == nil {
			t.Fatalf("expected malformed structure to fail: %s", sql)
		}
	}
}

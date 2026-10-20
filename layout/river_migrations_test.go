package layout

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/templates"
)

// TestRiverMigrationsMatchPinnedRiver compares the vendored River migrations
// with the main-line migrations of the River driver pinned in go_mod.tmpl. A
// River upgrade that adds or changes a migration fails here until the
// templates are updated.
func TestRiverMigrationsMatchPinnedRiver(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads the pinned River module")
	}

	goMod, err := fs.ReadFile(templates.Files, "go_mod.tmpl")
	if err != nil {
		t.Fatalf("read go_mod.tmpl: %v", err)
	}
	match := regexp.MustCompile(`github\.com/riverqueue/river/riverdriver/riverpgxv5 (v\S+)`).
		FindSubmatch(goMod)
	if match == nil {
		t.Fatal("go_mod.tmpl does not pin github.com/riverqueue/river/riverdriver/riverpgxv5")
	}
	module := "github.com/riverqueue/river/riverdriver/riverpgxv5@" + string(match[1])

	cmd := exec.Command("go", "mod", "download", "-json", module)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go mod download %s: %v", module, err)
	}
	var downloaded struct{ Dir string }
	if err := json.Unmarshal(out, &downloaded); err != nil || downloaded.Dir == "" {
		t.Fatalf("parse go mod download output %q: %v", out, err)
	}

	migrationDir := filepath.Join(downloaded.Dir, "migration", "main")
	ups, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != len(riverMigrations) {
		t.Fatalf(
			"%s has %d main-line migrations, the scaffold vendors %d: add the new ones to riverMigrations",
			module, len(ups), len(riverMigrations),
		)
	}

	for i, migration := range riverMigrations {
		content, err := fs.ReadFile(templates.Files, migration.template)
		if err != nil {
			t.Fatalf("read %s: %v", migration.template, err)
		}
		up, down, ok := strings.Cut(string(content), "-- +goose Down")
		if !ok {
			t.Fatalf("%s has no goose Down section", migration.template)
		}

		wantUp := readRiverMigration(t, ups[i])
		wantDown := readRiverMigration(t, strings.TrimSuffix(ups[i], ".up.sql")+".down.sql")
		if got := normalizeRiverSQL(up); got != wantUp {
			t.Errorf("%s up differs from %s:\n got: %s\nwant: %s", migration.template, ups[i], got, wantUp)
		}
		if got := normalizeRiverSQL(down); got != wantDown {
			t.Errorf("%s down differs from River:\n got: %s\nwant: %s", migration.template, got, wantDown)
		}
	}
}

func readRiverMigration(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return normalizeRiverSQL(string(content))
}

var riverSQLComment = regexp.MustCompile(`--[^\n]*`)

// normalizeRiverSQL drops comments (goose annotations included), River's
// schema placeholder and whitespace differences.
func normalizeRiverSQL(sql string) string {
	sql = strings.ReplaceAll(sql, "/* TEMPLATE: schema */", "")
	sql = riverSQLComment.ReplaceAllString(sql, "")

	return strings.Join(strings.Fields(sql), " ")
}

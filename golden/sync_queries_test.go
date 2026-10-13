package golden

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

func TestSyncQueries(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")

	// The fixture already colocates the widget SQL with its domain; sync
	// queries compiles domains/**/*.sql into domains/internal/queries via
	// narsilc and must not rewrite the hand-authored SQL.
	widgetSQL := filepath.Join(project, "domains", "widget", "widget.sql")
	before, err := os.ReadFile(widgetSQL)
	if err != nil {
		t.Fatalf("read fixture widget.sql: %v", err)
	}

	goldentest.RunCLI(t, project, "sync", "queries")

	g := goldentest.NewGoldie(t)
	goldentest.AssertDir(t, g, "sync/queries/widget_model", project, "domains/internal/queries")

	after, err := os.ReadFile(widgetSQL)
	if err != nil {
		t.Fatalf("read widget.sql after sync queries: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("sync queries rewrote the hand-authored domains/widget/widget.sql")
	}
}

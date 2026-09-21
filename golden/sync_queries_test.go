package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestSyncQueries(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")
	goldentest.CopyMigrations(t, project, "controller_view_generation")
	// Model generate writes annotated models/queries/*.sql; sync queries
	// compiles them into models/internal/queries via narsilc.
	goldentest.RunCLI(t, project, "generate", "model", "Widget", "--skip-factory")
	goldentest.RunCLI(t, project, "sync", "queries")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFiles(t, g, "sync/queries/widget_model", project, []string{
		"models/queries/widget.sql",
	})
	goldentest.AssertDir(t, g, "sync/queries/widget_model", project, "models/internal/queries")
}

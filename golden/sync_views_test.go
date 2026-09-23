package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

func TestSyncViews(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")
	goldentest.CopyMigrations(t, project, "controller_view_generation")
	goldentest.RunCLI(t, project, "generate", "scaffold", "Widget", "--skip-factory")
	goldentest.RunCLI(t, project, "sync", "views")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFiles(t, g, "sync/views/widget_scaffold", project, []string{
		"views/widgets_resource_templ.go",
	})
}

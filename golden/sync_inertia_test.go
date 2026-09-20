package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestSyncRoutes(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_inertia_vue")
	goldentest.CopyMigrations(t, project, "scaffold_generation_projects")
	goldentest.RunCLI(t, project, "generate", "scaffold", "Project", "--inertia", "--skip-factory")
	goldentest.RunCLI(t, project, "sync", "routes")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFiles(t, g, "sync/routes/inertia_vue", project, []string{
		"resources/js/routes.ts",
	})
}

func TestSyncPayloads(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_inertia_vue")
	goldentest.CopyMigrations(t, project, "scaffold_generation_projects")
	goldentest.RunCLI(t, project, "generate", "scaffold", "Project", "--inertia", "--skip-factory")
	goldentest.RunCLI(t, project, "sync", "payloads")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFiles(t, g, "sync/payloads/inertia_vue", project, []string{
		"resources/js/types/payloads.ts",
	})
}

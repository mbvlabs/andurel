package golden

import (
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

// slimScaffoldDirs are the top-level directories that define the new shape.
var slimScaffoldDirs = []string{
	"domains",
	"services",
	"handlers",
	"routes",
	"middleware",
	"resources/js/Pages",
}

func TestNewProjectInertia(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name string
		args []string
	}{
		{
			name: "inertia-react",
			args: []string{"new", "app"},
		},
		{
			name: "inertia-vue",
			args: []string{"new", "app", "--ui", "vue/pnpm"},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parent := t.TempDir()
			goldentest.RunCLI(t, parent, scenario.args...)

			project := filepath.Join(parent, "app")
			g := goldentest.NewGoldie(t)
			for _, dir := range slimScaffoldDirs {
				goldentest.AssertDir(t, g, "new/slim/"+scenario.name, project, dir)
			}
		})
	}
}

func TestNewProjectFullScaffold(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)

	scenarios := []struct {
		name string
		args []string
	}{
		{
			name: "postgresql-inertia-vue",
			args: []string{"new", "app", "--ui", "vue/pnpm"},
		},
		{
			name: "postgresql-inertia-react",
			args: []string{"new", "app"},
		},
		{
			name: "postgresql-inertia-svelte",
			args: []string{"new", "app", "--ui", "svelte/pnpm"},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parent := t.TempDir()
			goldentest.RunCLIFull(t, parent, scenario.args...)

			project := filepath.Join(parent, "app")
			goldentest.AssertTree(t, project, "new/"+scenario.name, goldentest.FullScaffoldDenylist)
		})
	}
}

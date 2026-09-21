package golden

import (
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestNewProjectMVC(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name string
		args []string
		dirs []string
	}{
		{
			name: "postgresql",
			args: []string{"new", "app"},
			dirs: []string{
				"models",
				"views",
				"controllers",
			},
		},
		{
			name: "postgresql-inertia-react",
			args: []string{"new", "app", "--inertia", "react"},
			dirs: []string{
				"models",
				"controllers",
				"resources/js/Pages",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parent := t.TempDir()
			goldentest.RunCLI(t, parent, scenario.args...)

			project := filepath.Join(parent, "app")
			g := goldentest.NewGoldie(t)
			for _, dir := range scenario.dirs {
				goldentest.AssertDir(t, g, "new/"+scenario.name, project, dir)
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
			name: "postgresql",
			args: []string{"new", "app"},
		},
		{
			name: "postgresql-inertia-vue",
			args: []string{"new", "app", "--inertia", "vue"},
		},
		{
			name: "postgresql-inertia-react",
			args: []string{"new", "app", "--inertia", "react"},
		},
		{
			name: "postgresql-inertia-svelte",
			args: []string{"new", "app", "--inertia", "svelte"},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parent := t.TempDir()
			goldentest.RunCLI(t, parent, scenario.args...)

			project := filepath.Join(parent, "app")
			goldentest.AssertTree(t, project, "new/"+scenario.name, goldentest.FullScaffoldDenylist)
		})
	}
}

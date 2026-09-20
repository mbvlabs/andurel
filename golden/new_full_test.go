package golden

import (
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

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

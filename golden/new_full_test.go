package golden

import (
	"os"
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
			assertNoGeneratedDrift(t, project)
		})
	}
}

// inertiaGeneratedFiles maps each sync-owned frontend file to the command
// that regenerates it. andurel verify's generated step fails on drift in any
// of them.
var inertiaGeneratedFiles = []struct {
	path string
	sync string
}{
	{path: "resources/js/routes.ts", sync: "routes"},
	{path: "resources/js/types/payloads.ts", sync: "payloads"},
}

// assertNoGeneratedDrift re-runs the sync generators on a fresh scaffold and
// requires byte-identical output, so andurel new never ships a project that
// andurel verify rejects.
func assertNoGeneratedDrift(t *testing.T, project string) {
	t.Helper()

	for _, file := range inertiaGeneratedFiles {
		scaffolded, err := os.ReadFile(filepath.Join(project, file.path))
		if err != nil {
			t.Fatalf("andurel new did not write %s: %v", file.path, err)
		}
		goldentest.RunCLI(t, project, "sync", file.sync)
		goldentest.AssertFileBytesEquals(t, project, file.path, scaffolded)
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
			goldentest.SeedProjectTools(t, project)
			goldentest.RunCLI(t, project, "verify")
		})
	}
}

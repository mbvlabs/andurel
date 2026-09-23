package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

type generateStep struct {
	addMigrations string
	args          []string
}

func runGenerateSteps(t *testing.T, project string, steps []generateStep) {
	t.Helper()
	for _, step := range steps {
		if step.addMigrations != "" {
			goldentest.CopyMigrations(t, project, step.addMigrations)
		}
		goldentest.RunCLI(t, project, step.args...)
	}
}

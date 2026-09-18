package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestGenerateJobSmoke(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "job_smoke")
	goldentest.RunCLI(t, project, "generate", "job", "Smoke")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFile(t, g, "smoke/generate_job/queue/jobs/smoke.go", project, "queue/jobs/smoke.go")
}

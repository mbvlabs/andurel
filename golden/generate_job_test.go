package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestGenerateJob(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name    string
		args    []string
		capture []string
	}{
		{
			name: "default_queue",
			args: []string{"generate", "job", "Smoke"},
			capture: []string{
				"queue/jobs/smoke.go",
				"queue/smoke.go",
				"queue/workers.go",
			},
		},
		{
			name: "named_queue",
			args: []string{"generate", "job", "ProcessPayment", "--queue", "financial"},
			capture: []string{
				"queue/jobs/process_payment.go",
				"queue/process_payment.go",
				"queue/workers.go",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			project := goldentest.CopyFixture(t, "job_smoke")
			goldentest.RunCLI(t, project, scenario.args...)

			g := goldentest.NewGoldie(t)
			goldentest.AssertFiles(t, g, "generate/job/"+scenario.name, project, scenario.capture)
		})
	}
}

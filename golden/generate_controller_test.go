package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestGenerateController(t *testing.T) {
	goldentest.RequireBinary(t)

	modelStep := generateStep{
		args: []string{"generate", "model", "Widget", "--skip-factory"},
	}

	scenarios := []struct {
		name    string
		steps   []generateStep
		capture []string
	}{
		{
			name: "full_crud",
			steps: []generateStep{
				modelStep,
				{args: []string{"generate", "controller", "Widget"}},
			},
			capture: []string{
				"controllers/widgets.go",
				"router/routes/widgets.go",
				"controllers/controller.go",
				"views/widgets_resource.templ",
			},
		},
		{
			name: "single_action",
			steps: []generateStep{
				modelStep,
				{args: []string{"generate", "controller", "Widget", "show"}},
			},
			capture: []string{
				"controllers/widgets.go",
				"router/routes/widgets.go",
				"controllers/controller.go",
				"views/widgets_resource.templ",
			},
		},
		{
			name: "add_action",
			steps: []generateStep{
				modelStep,
				{args: []string{"generate", "controller", "Widget", "show"}},
				{args: []string{"generate", "controller", "Widget", "edit"}},
			},
			capture: []string{
				"controllers/widgets.go",
				"router/routes/widgets.go",
				"controllers/controller.go",
				"views/widgets_resource.templ",
			},
		},
		{
			name: "model_name",
			steps: []generateStep{
				modelStep,
				{args: []string{
					"generate", "controller", "Dashboard",
					"--model-name", "Widget",
					"index", "show",
				}},
			},
			capture: []string{
				"controllers/dashboards.go",
				"router/routes/dashboards.go",
				"controllers/controller.go",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			project := goldentest.CopyFixture(t, "generate_base")
			goldentest.CopyMigrations(t, project, "controller_view_generation")

			g := goldentest.NewGoldie(t)
			runGenerateSteps(t, project, scenario.steps)
			goldentest.AssertFiles(t, g, "generate/controller/"+scenario.name, project, scenario.capture)
		})
	}
}

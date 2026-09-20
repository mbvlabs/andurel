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
		fixture string
		steps   []generateStep
		capture []string
		missing []string
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
		{
			name: "namespaced",
			steps: []generateStep{
				modelStep,
				{args: []string{"generate", "controller", "admin/Widget"}},
			},
			capture: []string{
				"controllers/admin/widgets.go",
				"router/routes/admin_widgets.go",
				"views/admin_widgets_resource.templ",
				"controllers/controller.go",
			},
		},
		{
			name:    "inertia_vue",
			fixture: "generate_inertia_vue",
			steps: []generateStep{
				modelStep,
				{args: []string{"generate", "controller", "Widget", "--inertia"}},
			},
			capture: []string{
				"controllers/widgets.go",
				"router/routes/widgets.go",
				"controllers/controller.go",
				"resources/js/Pages/Widget/Index.vue",
				"resources/js/Pages/Widget/Show.vue",
				"resources/js/Pages/Widget/Create.vue",
				"resources/js/Pages/Widget/Edit.vue",
				"resources/js/routes.ts",
				"resources/js/types/payloads.ts",
			},
			missing: []string{
				"views/widgets_resource.templ",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			fixture := scenario.fixture
			if fixture == "" {
				fixture = "generate_base"
			}
			project := goldentest.CopyFixture(t, fixture)
			goldentest.CopyMigrations(t, project, "controller_view_generation")

			g := goldentest.NewGoldie(t)
			runGenerateSteps(t, project, scenario.steps)
			goldentest.AssertFiles(t, g, "generate/controller/"+scenario.name, project, scenario.capture)
			for _, path := range scenario.missing {
				goldentest.AssertMissing(t, project, path)
			}
		})
	}
}

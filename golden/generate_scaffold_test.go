package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestGenerateScaffold(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name       string
		fixture    string
		migrations string
		args       []string
		capture    []string
		missing    []string
	}{
		{
			name:       "full_crud",
			fixture:    "generate_base",
			migrations: "controller_view_generation",
			args:       []string{"generate", "scaffold", "Widget"},
			capture: []string{
				"models/model.go",
				"models/widget.go",
				"models/factories/widget.go",
				"controllers/controller.go",
				"controllers/widgets.go",
				"router/routes/widgets.go",
				"views/widgets_resource.templ",
			},
		},
		{
			name:       "skip_factory",
			fixture:    "generate_base",
			migrations: "controller_view_generation",
			args:       []string{"generate", "scaffold", "Widget", "--skip-factory"},
			capture: []string{
				"models/widget.go",
				"controllers/widgets.go",
				"router/routes/widgets.go",
				"views/widgets_resource.templ",
			},
			missing: []string{
				"models/factories/widget.go",
			},
		},
		{
			name:       "table_name_override",
			fixture:    "generate_base",
			migrations: "scaffold_generation_student_feedback",
			args: []string{
				"generate", "scaffold", "FeedbackEntry",
				"--table-name", "student_feedback",
				"--skip-factory",
			},
			capture: []string{
				"models/feedback_entry.go",
				"controllers/student_feedback.go",
				"router/routes/student_feedback.go",
				"views/student_feedback_resource.templ",
			},
		},
		{
			name:       "irregular_plural",
			fixture:    "generate_base",
			migrations: "scaffold_generation_companies",
			args:       []string{"generate", "scaffold", "Company"},
			capture: []string{
				"models/company.go",
				"models/factories/company.go",
				"controllers/companies.go",
				"router/routes/companies.go",
				"views/companies_resource.templ",
			},
		},
		{
			name:       "array_fields",
			fixture:    "generate_base",
			migrations: "scaffold_generation_documents",
			args:       []string{"generate", "scaffold", "Document", "--skip-factory"},
			capture: []string{
				"models/document.go",
				"controllers/documents.go",
				"router/routes/documents.go",
				"views/documents_resource.templ",
			},
		},
		{
			name:       "custom_primary_key",
			fixture:    "generate_base",
			migrations: "scaffold_generation_warehouses",
			args: []string{
				"generate", "scaffold", "Warehouse",
				"--primary-key", "slug",
				"--skip-factory",
			},
			capture: []string{
				"models/warehouse.go",
				"controllers/warehouses.go",
				"router/routes/warehouses.go",
				"views/warehouses_resource.templ",
			},
		},
		{
			name:       "vue",
			fixture:    "generate_inertia_vue",
			migrations: "scaffold_generation_projects",
			args: []string{
				"generate", "scaffold", "Project",
				"--skip-factory",
			},
			capture: []string{
				"models/project.go",
				"controllers/projects.go",
				"router/routes/projects.go",
				"resources/js/Pages/Project/Index.vue",
				"resources/js/Pages/Project/Show.vue",
				"resources/js/Pages/Project/Create.vue",
				"resources/js/Pages/Project/Edit.vue",
				"resources/js/routes.ts",
				"resources/js/types/payloads.ts",
			},
			missing: []string{
				"views/projects_resource.templ",
			},
		},
		{
			name:       "api",
			fixture:    "generate_base",
			migrations: "controller_view_generation",
			args: []string{
				"generate", "scaffold", "Widget",
				"--api",
				"--skip-factory",
			},
			capture: []string{
				"models/widget.go",
				"controllers/api/widgets.go",
				"router/routes/api_widgets.go",
				"controllers/controller.go",
			},
			missing: []string{
				"views/widgets_resource.templ",
				"controllers/widgets.go",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			project := goldentest.CopyFixture(t, scenario.fixture)
			goldentest.CopyMigrations(t, project, scenario.migrations)

			goldentest.RunCLI(t, project, scenario.args...)

			g := goldentest.NewGoldie(t)
			goldentest.AssertFiles(t, g, "generate/scaffold/"+scenario.name, project, scenario.capture)
			for _, path := range scenario.missing {
				goldentest.AssertMissing(t, project, path)
			}
		})
	}
}

package golden

import (
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

// newProjectCapture is a curated allowlist of scaffolded source files that are
// byte-stable under -tags andurel_golden (seeded secrets + fixed migration
// timestamps). Paths that need network or float without scrubbing are omitted:
// bin/, go.sum, .git/, models/internal/queries/, and templ-generated *_templ.go
// beyond what templates already emit. See testdata/golden/README.md.
var newProjectCapture = []string{
	"go.mod",
	".env.example",
	"andurel.lock",
	"cmd/app/main.go",
	"config/config.go",
	"config/database.go",
	"config/app.go",
	"controllers/controller.go",
	"controllers/pages.go",
	"models/model.go",
	"models/user.go",
	"models/token.go",
	"models/queries/user.sql",
	"models/queries/token.sql",
	"models/factories/user.go",
	"router/router.go",
	"router/routes/pages.go",
	"router/routes/users.go",
	"services/service.go",
	"services/authentication.go",
	"queue/queue.go",
	"views/layout.templ",
	"views/welcome.templ",
	"migrations/20250101000000_create_river_migration_table.sql",
	"migrations/20250101000001_create_river_job_and_leader_tables.sql",
	"migrations/20250101000006_create_users_table.sql",
	"migrations/20250101000007_create_tokens_table.sql",
}

var newProjectInertiaVueCapture = []string{
	"go.mod",
	".env.example",
	"andurel.lock",
	"cmd/app/main.go",
	"cmd/ssr/main.go",
	"config/config.go",
	"config/inertia.go",
	"controllers/controller.go",
	"controllers/pages.go",
	"models/model.go",
	"models/user.go",
	"package.json",
	"vite.config.ts",
	"tsconfig.json",
	"resources/js/app.ts",
	"resources/js/ssr.ts",
	"resources/js/routes.ts",
	"resources/js/Layouts/Layout.vue",
	"resources/js/Pages/Auth/Login.vue",
	"views/root.templ",
	"migrations/20250101000006_create_users_table.sql",
}

func TestNewProject(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name    string
		args    []string
		capture []string
		missing []string
	}{
		{
			name:    "postgresql",
			args:    []string{"new", "app"},
			capture: newProjectCapture,
		},
		{
			name:    "postgresql-inertia-vue",
			args:    []string{"new", "app", "--inertia", "vue"},
			capture: newProjectInertiaVueCapture,
			missing: []string{
				"views/login.templ",
				"views/registration.templ",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parent := t.TempDir()
			goldentest.RunCLI(t, parent, scenario.args...)

			project := filepath.Join(parent, "app")
			g := goldentest.NewGoldie(t)
			goldentest.AssertFiles(t, g, "new/"+scenario.name, project, scenario.capture)
			for _, path := range scenario.missing {
				goldentest.AssertMissing(t, project, path)
			}
		})
	}
}

package e2e

import (
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
)

type ScaffoldConfig struct {
	Name       string
	Database   string
	CSS        string
	Extensions []string
	Critical   bool
}

func getScaffoldConfigs() []ScaffoldConfig {
	return []ScaffoldConfig{
		{
			Name:     "postgresql-tailwind",
			Database: "postgresql",
			CSS:      "tailwind",
			Critical: true,
		},
		{
			Name:     "postgresql-vanilla",
			Database: "postgresql",
			CSS:      "vanilla",
			Critical: true,
		},
		{
			Name:     "sqlite-tailwind",
			Database: "sqlite",
			CSS:      "tailwind",
			Critical: true,
		},
		{
			Name:     "sqlite-vanilla",
			Database: "sqlite",
			CSS:      "vanilla",
			Critical: true,
		},
		{
			Name:       "postgresql-tailwind-auth",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"auth"},
			Critical:   true,
		},
		{
			Name:       "sqlite-vanilla-docker",
			Database:   "sqlite",
			CSS:        "vanilla",
			Extensions: []string{"docker"},
			Critical:   true,
		},
		{
			Name:       "postgresql-tailwind-email",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"email"},
			Critical:   false,
		},
		{
			Name:       "sqlite-tailwind-auth",
			Database:   "sqlite",
			CSS:        "tailwind",
			Extensions: []string{"auth"},
			Critical:   false,
		},
		{
			Name:       "postgresql-vanilla-auth",
			Database:   "postgresql",
			CSS:        "vanilla",
			Extensions: []string{"auth"},
			Critical:   false,
		},
		{
			Name:       "sqlite-vanilla-auth",
			Database:   "sqlite",
			CSS:        "vanilla",
			Extensions: []string{"auth"},
			Critical:   false,
		},
		{
			Name:       "postgresql-tailwind-all-extensions",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"auth", "email", "docker"},
			Critical:   false,
		},
		{
			Name:       "sqlite-vanilla-all-extensions",
			Database:   "sqlite",
			CSS:        "vanilla",
			Extensions: []string{"auth", "email", "docker"},
			Critical:   false,
		},
	}
}

func TestScaffoldMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E scaffold matrix test in short mode")
	}

	binary := buildAndurelBinary(t)

	configs := getScaffoldConfigs()

	for _, config := range configs {
		config := config
		t.Run(config.Name, func(t *testing.T) {
			if isCriticalOnly() && !config.Critical {
				t.Skip("Skipping non-critical test in critical-only mode")
			}

			t.Parallel()

			project := internal.NewProject(t, binary)

			args := []string{
				"-d", config.Database,
				"-c", config.CSS,
			}

			if len(config.Extensions) > 0 {
				for _, ext := range config.Extensions {
					args = append(args, "-e", ext)
				}
			}

			err := project.Scaffold(args...)
			internal.AssertCommandSucceeds(t, err, "scaffold")

			verifyScaffoldedProject(t, project, config)

			internal.AssertGoVetPasses(t, project)
		})
	}
}

func verifyScaffoldedProject(t *testing.T, project *internal.Project, config ScaffoldConfig) {
	t.Helper()

	coreFiles := []string{
		"go.mod",
		"go.sum",
		".env.example",
		".gitignore",
		"main.go",
		"Makefile",
		"sqlc.yaml",
		"cmd/app/main.go",
		"cmd/console/main.go",
		"cmd/migration/main.go",
		"cmd/run/main.go",
		"controllers/home_controller.go",
		"database/db.go",
		"routes/routes.go",
		"views/layouts/base.templ",
		"views/home/index.templ",
	}
	internal.AssertFilesExist(t, project, coreFiles)

	if config.Database == "postgresql" {
		internal.AssertFileExists(t, project, "database/migrations/000001_create_river_tables.up.sql")
		internal.AssertFileExists(t, project, "database/migrations/000001_create_river_tables.down.sql")
	}

	if config.Database == "sqlite" {
		internal.AssertFileExists(t, project, ".env")
	}

	if config.CSS == "tailwind" {
		internal.AssertDirExists(t, project, "css")
		internal.AssertFileExists(t, project, "tailwind.config.js")
	}

	if config.CSS == "vanilla" {
		internal.AssertDirExists(t, project, "assets/css")
	}

	for _, ext := range config.Extensions {
		verifyExtension(t, project, ext)
	}
}

func verifyExtension(t *testing.T, project *internal.Project, extension string) {
	t.Helper()

	switch extension {
	case "auth":
		authFiles := []string{
			"controllers/auth_controller.go",
			"controllers/sessions_controller.go",
			"models/user.go",
			"models/token.go",
			"views/auth/login.templ",
			"views/auth/register.templ",
		}
		internal.AssertFilesExist(t, project, authFiles)

	case "email":
		emailFiles := []string{
			"pkg/email/client.go",
			"views/emails/layouts/base.templ",
		}
		internal.AssertFilesExist(t, project, emailFiles)

	case "docker":
		dockerFiles := []string{
			"Dockerfile",
			".dockerignore",
		}
		internal.AssertFilesExist(t, project, dockerFiles)
	}
}

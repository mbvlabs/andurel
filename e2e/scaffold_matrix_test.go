package e2e

import (
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
)

type ScaffoldConfig struct {
	Name       string
	Database   string
	CSS        string
	DIMode     string
	Inertia    string
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
			Name:       "postgresql-vanilla-css-components",
			Database:   "postgresql",
			CSS:        "vanilla",
			Extensions: []string{"css-components"},
			Critical:   true,
		},
		{
			Name:       "postgresql-tailwind-all-extensions",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"docker"},
			Critical:   true,
		},
		{
			Name:       "postgresql-tailwind-aws-ses",
			Database:   "postgresql",
			CSS:        "tailwind",
			Extensions: []string{"aws-ses"},
			Critical:   true,
		},
		{
			Name:     "postgresql-tailwind-uberfx",
			Database: "postgresql",
			CSS:      "tailwind",
			DIMode:   "uberfx",
			Critical: true,
		},
		{
			Name:     "postgresql-tailwind-inertia-vue",
			Database: "postgresql",
			CSS:      "tailwind",
			Inertia:  "vue",
			Critical: true,
		},
		{
			Name:       "postgresql-vanilla-uberfx-all-extensions",
			Database:   "postgresql",
			CSS:        "vanilla",
			DIMode:     "uberfx",
			Extensions: []string{"docker", "aws-ses", "css-components"},
			Critical:   true,
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
		t.Run(config.Name, func(t *testing.T) {
			if isCriticalOnly() && !config.Critical {
				t.Skip("Skipping non-critical test in critical-only mode")
			}

			t.Parallel()

			project := internal.NewProject(t, binary, getSharedBinDir())

			args := []string{
				"-c", config.CSS,
			}

			if config.DIMode != "" {
				args = append(args, "--di", config.DIMode)
			}

			if config.Inertia != "" {
				args = append(args, "--inertia", config.Inertia)
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
		"cmd/app/main.go",
		"controllers/pages.go",
		"views/layout.templ",
		"views/home.templ",
	}
	internal.AssertFilesExist(t, project, coreFiles)

	internal.AssertFileExists(
		t,
		project,
		"database/migrations/00001_create_river_migration_table.sql",
	)
	internal.AssertFileExists(
		t,
		project,
		"database/migrations/00002_create_river_job_and_leader_tables.sql",
	)

	if config.CSS == "tailwind" {
		internal.AssertDirExists(t, project, "assets/css")
	}

	if config.CSS == "vanilla" {
		internal.AssertDirExists(t, project, "assets/css")
		vanillaCSSFiles := []string{
			"assets/css/style.css",
			"assets/css/reset.css",
			"assets/css/tokens.css",
			"assets/css/base.css",
			"assets/css/objects.css",
			"assets/css/utilities.css",
		}
		internal.AssertFilesExist(t, project, vanillaCSSFiles)
		internal.AssertFileContains(t, project, "router/routes/assets.go", `"/css/*"`)
		internal.AssertFileContains(t, project, "controllers/assets.go", `etx.Param("*")`)
	}

	if config.DIMode == "uberfx" {
		internal.AssertFileContains(t, project, "cmd/app/main.go", "fx.New")
		internal.AssertFileContains(t, project, "cmd/app/main.go", "fx.Provide")
	} else {
		internal.AssertFileContains(t, project, "cmd/app/main.go", "func run()")
	}

	if config.Inertia == "vue" {
		inertiaFiles := []string{
			"resources/js/app.ts",
			"resources/js/Pages/Welcome.vue",
			"vite.config.ts",
			"package.json",
			"tsconfig.json",
			"views/root.go.html",
			"internal/renderer/vite.go",
		}
		internal.AssertFilesExist(t, project, inertiaFiles)
	} else {
		internal.AssertFileContains(t, project, "controllers/pages.go", "views.Home")
	}

	// Auth is now part of base scaffold, verify auth files always exist
	authFiles := []string{
		"controllers/sessions.go",
		"controllers/registrations.go",
		"controllers/reset_passwords.go",
		"controllers/confirmations.go",
		"models/user.go",
		"models/token.go",
		"views/login.templ",
		"views/registration.templ",
		"views/reset_password.templ",
		"views/confirm_email.templ",
	}
	internal.AssertFilesExist(t, project, authFiles)

	for _, ext := range config.Extensions {
		verifyExtension(t, project, ext)
	}
}

func verifyExtension(t *testing.T, project *internal.Project, extension string) {
	t.Helper()

	switch extension {
	case "email":
		emailFiles := []string{
			"email/email.go",
			"email/base_layout.templ",
			"clients/mail_hog.go",
			"config/email.go",
		}
		internal.AssertFilesExist(t, project, emailFiles)

	case "docker":
		dockerFiles := []string{
			"Dockerfile",
			".dockerignore",
		}
		internal.AssertFilesExist(t, project, dockerFiles)

	case "aws-ses":
		awsSesFiles := []string{
			"clients/email/aws_ses.go",
			"config/aws_ses.go",
		}
		internal.AssertFilesExist(t, project, awsSesFiles)

	case "css-components":
		cssComponentFiles := []string{
			"assets/css/components/layout.css",
			"assets/css/components/panels.css",
			"assets/css/components/buttons.css",
			"assets/css/components/forms.css",
			"assets/css/components/feedback.css",
		}
		internal.AssertFilesExist(t, project, cssComponentFiles)
	}
}

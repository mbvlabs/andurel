package simpleauth

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/mbvlabs/andurel/layout/extensions"
)

type Extension struct{}

func (e Extension) Name() string {
	return "simple-auth"
}

func (e Extension) Apply(ctx *extensions.Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("simple-auth: context or data is nil")
	}

	builder := ctx.Builder()
	if builder == nil {
		return fmt.Errorf("simple-auth: builder is nil")
	}

	moduleName := ctx.Data.GetModuleName()
	database := ctx.Data.DatabaseDialect()

	// Add Auth controller to blueprint
	if err := e.addAuthController(builder, moduleName); err != nil {
		return fmt.Errorf("simple-auth: failed to add controller: %w", err)
	}

	// Render all template files
	if err := e.renderTemplates(ctx, database); err != nil {
		return fmt.Errorf("simple-auth: failed to render templates: %w", err)
	}

	// Schedule sqlc generation to run after all template rendering
	e.addSqlcPostStep(ctx)

	return nil
}

func (e Extension) addAuthController(builder extensions.Builder, moduleName string) error {
	// Add config import - needed for the cfg config.Config dependency parameter
	builder.AddImport(fmt.Sprintf("%s/config", moduleName))

	// Add config dependency to controller constructor
	builder.AddControllerDependency("cfg", "config.Config")

	// Add Auth controller field
	builder.AddControllerField("Auth", "Auth")

	// Add Auth constructor
	builder.AddConstructor("auth", "newAuth(cfg, db)")

	// Register auth route group for aggregation in router_routes_routes.tmpl
	builder.AddRouteGroup("auth")

	// Add model imports for validator and crypto dependencies
	builder.AddModelImport("github.com/google/uuid")
	builder.AddModelImport("golang.org/x/crypto/argon2")

	return nil
}

func (e Extension) renderTemplates(ctx *extensions.Context, database string) error {
	templates := map[string]string{
		"controllers_auth.tmpl":    "controllers/auth.go",
		"middleware_auth.tmpl":     "router/middleware/auth.go",
		"router_cookies_auth.tmpl": "router/cookies/auth.go",
		"router_routes_auth.tmpl":  "router/routes/auth.go",
		"models_user.tmpl":         "models/user.go",
		"queries_users.tmpl":       "database/queries/users.sql",
		"views_auth_login.tmpl":    "views/auth/login.templ",
		"views_auth_signup.tmpl":   "views/auth/signup.templ",
	}

	// Determine which migration to use
	var migrationTemplate string
	if database == "postgresql" {
		migrationTemplate = "migration_001_users_pg.tmpl"
	} else {
		migrationTemplate = "migration_001_users_sqlite.tmpl"
	}

	templates[migrationTemplate] = fmt.Sprintf(
		"database/migrations/%s_create_users_table.sql",
		time.Now().Format("20060102150405"),
	)

	// Process each template
	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("simple-auth/templates/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}

var (
	sqlcPostStepAdded bool
	sqlcPostStepMu    sync.Mutex
)

// addSqlcPostStep schedules the sqlc generation to run after all templates are rendered.
// This ensures the models/internal/db package is generated so the project compiles.
// Uses a mutex to ensure it's only added once even if called multiple times.
func (e Extension) addSqlcPostStep(ctx *extensions.Context) {
	sqlcPostStepMu.Lock()
	defer sqlcPostStepMu.Unlock()

	if sqlcPostStepAdded {
		return
	}

	ctx.AddPostStep(func() error {
		cmd := exec.Command("go", "tool", "sqlc", "generate", "-f", "database/sqlc.yaml")
		cmd.Dir = ctx.TargetDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("simple-auth: failed to run sqlc generate: %w", err)
		}
		return nil
	})

	sqlcPostStepAdded = true
}

func Register() error {
	return extensions.Register(Extension{})
}

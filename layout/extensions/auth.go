package extensions

import (
	"fmt"
	"time"

	"github.com/mbvlabs/andurel/layout/cmds"
)

type Auth struct{}

func (e Auth) Name() string {
	return "auth"
}

func (e Auth) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("auth: context or data is nil")
	}

	builder := ctx.Builder()

	moduleName := ctx.Data.GetModuleName()

	builder.AddControllerImport(fmt.Sprintf("%s/config", moduleName))

	builder.AddControllerDependency("cfg", "config.Config")

	builder.AddConfigField("Auth", "auth")

	builder.AddControllerField("Sessions", "Sessions")
	builder.AddControllerField("Registrations", "Registrations")
	builder.AddControllerField("Confirmations", "Confirmations")
	builder.AddControllerField("ResetPassword", "ResetPassword")

	builder.AddControllerConstructor("sessions", "newSessions(db, cfg)")
	builder.AddControllerConstructor("registrations", "newRegistrations(db, emailClient, cfg)")
	builder.AddControllerConstructor("confirmations", "newConfirmations(db, cfg)")
	builder.AddControllerConstructor("resetPassword", "newResetPassword(db, emailClient, cfg)")

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("auth: failed to render templates: %w", err)
	}

	ctx.AddPostStep(func(targetDir string) error {
		return cmds.RunSqlcGenerate(targetDir)
	})

	return nil
}

func (e Auth) Dependencies() []string {
	return []string{Email{}.Name()}
}

func (e Auth) renderTemplates(ctx *Context) error {
	templates := map[string]string{
		"controllers_confirmations.tmpl":   "controllers/confirmations.go",
		"controllers_registrations.tmpl":   "controllers/registrations.go",
		"controllers_reset_passwords.tmpl": "controllers/reset_passwords.go",
		"controllers_sessions.tmpl":        "controllers/sessions.go",

		"config_auth.tmpl": "config/auth.go",

		"database_migrations_users.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_users_table.sql",
			time.Now().Format("20060102150405"),
		),
		"database_migrations_tokens.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_tokens_table.sql",
			time.Now().Add(1*time.Second).Format("20060102150405"),
		),
		"database_queries_tokens.tmpl": "database/queries/tokens.sql",
		"database_queries_users.tmpl":  "database/queries/users.sql",

		"email_reset_password.tmpl": "email/reset_password.templ",
		"email_verify_email.tmpl":   "email/verify_email.templ",
		"email_auth.tmpl":           "email/auth.go",

		"models_token.tmpl": "models/token.go",
		"models_user.tmpl":  "models/user.go",

		"models_interal_db_token_constructors.tmpl": "models/internal/db/token_constructors.go",
		"models_interal_db_user_constructors.tmpl":  "models/internal/db/user_constructors.go",

		"services_authentication.tmpl": "services/authentication.go",
		"services_registration.tmpl":   "services/registration.go",
		"services_reset_password.tmpl": "services/reset_password.go",

		"router_routes_users.tmpl": "router/routes/users.go",

		"views_confirm_email.tmpl":  "views/confirm_email.templ",
		"views_login.tmpl":          "views/login.templ",
		"views_registration.tmpl":   "views/registration.templ",
		"views_reset_password.tmpl": "views/reset_password.templ",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/auth/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}

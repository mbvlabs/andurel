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
	builder.AddControllerField("ResetPasswords", "ResetPasswords")

	builder.AddControllerConstructor("sessions", "newSessions(db, cfg)")
	builder.AddControllerConstructor("registrations", "newRegistrations(db, insertOnly, cfg)")
	builder.AddControllerConstructor("confirmations", "newConfirmations(db, cfg)")
	builder.AddControllerConstructor("resetPasswords", "newResetPasswords(db, insertOnly, cfg)")

	builder.AddCookiesImport("github.com/google/uuid")
	builder.AddCookiesImport(fmt.Sprintf("%s/models", moduleName))

	builder.AddCookiesConstant("isAuthenticated", "is_authenticated")
	builder.AddCookiesConstant("isAdmin", "is_admin")
	builder.AddCookiesConstant("userID", "user_id")

	builder.AddCookiesAppField("UserID", "uuid.UUID")
	builder.AddCookiesAppField("IsAdmin", "bool")
	builder.AddCookiesAppField("IsAuthenticated", "bool")

	builder.SetCookiesCreateSessionCode(`	sess.Values[isAuthenticated] = true
	sess.Values[isAdmin] = user.IsAdmin
	sess.Values[userID] = user.ID.String()`)

	builder.SetCookiesGetSessionCode(`	if v, ok := sess.Values[isAuthenticated].(bool); ok {
		app.IsAuthenticated = v
	}
	if v, ok := sess.Values[isAdmin].(bool); ok {
		app.IsAdmin = v
	}
	if v, ok := sess.Values[userID].(string); ok {
		app.UserID, _ = uuid.Parse(v)
	}`)

	builder.StartRouteRegistrationFunction("registerAuthRoutes", "sessionsController")
	builder.AddRouteRegistration("http.MethodGet", "routes.SessionNew", "ctrls.Sessions.New")
	builder.AddRouteRegistration("http.MethodPost", "routes.SessionCreate", "ctrls.Sessions.Create")
	builder.AddRouteRegistration(
		"http.MethodDelete",
		"routes.SessionDestroy",
		"ctrls.Sessions.Destroy",
	)
	builder.AddRouteRegistration("http.MethodGet", "routes.PasswordNew", "ctrls.ResetPasswords.New")
	builder.AddRouteRegistration(
		"http.MethodPost",
		"routes.PasswordCreate",
		"ctrls.ResetPasswords.Create",
	)
	builder.AddRouteRegistration(
		"http.MethodGet",
		"routes.PasswordEdit",
		"ctrls.ResetPasswords.Edit",
	)
	builder.AddRouteRegistration(
		"http.MethodPut",
		"routes.PasswordUpdate",
		"ctrls.ResetPasswords.Update",
	)
	builder.AddRouteRegistration(
		"http.MethodGet",
		"routes.RegistrationNew",
		"ctrls.Registrations.New",
	)
	builder.AddRouteRegistration(
		"http.MethodPost",
		"routes.RegistrationCreate",
		"ctrls.Registrations.Create",
	)
	builder.AddRouteRegistration(
		"http.MethodGet",
		"routes.ConfirmationNew",
		"ctrls.Confirmations.New",
	)
	builder.AddRouteRegistration(
		"http.MethodPost",
		"routes.ConfirmationCreate",
		"ctrls.Confirmations.Create",
	)
	builder.EndRouteRegistrationFunction()

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("auth: failed to render templates: %w", err)
	}

	ctx.AddPostStep(func(targetDir string) error {
		return cmds.RunSqlcGenerate(targetDir)
	})

	return nil
}

func (e Auth) Dependencies() []string {
	return nil
}

func (e Auth) renderTemplates(ctx *Context) error {
	baseTime := time.Now()
	if ctx.NextMigrationTime != nil && !ctx.NextMigrationTime.IsZero() {
		baseTime = *ctx.NextMigrationTime
	}

	templates := map[string]string{
		"controllers_confirmations.tmpl":   "controllers/confirmations.go",
		"controllers_registrations.tmpl":   "controllers/registrations.go",
		"controllers_reset_passwords.tmpl": "controllers/reset_passwords.go",
		"controllers_sessions.tmpl":        "controllers/sessions.go",

		"config_auth.tmpl": "config/auth.go",

		"database_migrations_users.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_users_table.sql",
			baseTime.Format("20060102150405"),
		),
		"database_migrations_tokens.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_tokens_table.sql",
			baseTime.Add(1*time.Second).Format("20060102150405"),
		),
		"database_queries_tokens.tmpl": "database/queries/tokens.sql",
		"database_queries_users.tmpl":  "database/queries/users.sql",

		"email_reset_password.tmpl": "email/reset_password.templ",
		"email_verify_email.tmpl":   "email/verify_email.templ",

		"models_token.tmpl": "models/token.go",
		"models_user.tmpl":  "models/user.go",

		"models_interal_db_token_constructors.tmpl": "models/internal/db/token_constructors.go",
		"models_interal_db_user_constructors.tmpl":  "models/internal/db/user_constructors.go",

		"services_authentication.tmpl": "services/authentication.go",
		"services_registration.tmpl":   "services/registration.go",
		"services_reset_password.tmpl": "services/reset_password.go",

		"router_routes_users.tmpl":    "router/routes/users.go",
		"router_middleware_auth.tmpl": "router/middleware/auth.go",

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

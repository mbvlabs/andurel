package auth

import (
	"fmt"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout"
)

func ProcessAuthRecipe(targetDir string, data layout.TemplateData) error {
	mappings := getTemplateMappings(data.Database)

	for tmplFile, targetPath := range mappings {
		fullTmplPath := filepath.Join("auth", "templates", tmplFile)
		if err := layout.ProcessTemplateFromRecipe(targetDir, fullTmplPath, targetPath, data); err != nil {
			return fmt.Errorf("failed to process auth template %s: %w", tmplFile, err)
		}
	}

	return nil
}

func getTemplateMappings(database string) map[string]string {
	migrationSuffix := "_pg.tmpl"
	if database == "sqlite" {
		migrationSuffix = "_sqlite.tmpl"
	}

	return map[string]string{
		"migration_001_users" + migrationSuffix:  "database/migrations/001_users.sql",
		"migration_002_tokens" + migrationSuffix: "database/migrations/002_tokens.sql",

		"queries_users.tmpl":  "database/queries/users.sql",
		"queries_tokens.tmpl": "database/queries/tokens.sql",

		"models_user.tmpl":  "models/user.go",
		"models_token.tmpl": "models/token.go",

		"services_email.tmpl": "services/email.go",

		"middleware_auth.tmpl": "router/middleware/auth.go",

		"controllers_auth.tmpl": "controllers/auth.go",

		"views_auth_login.tmpl":               "views/auth/login.templ",
		"views_auth_signup.tmpl":              "views/auth/signup.templ",
		"views_auth_forgot_password.tmpl":     "views/auth/forgot_password.templ",
		"views_auth_reset_password.tmpl":      "views/auth/reset_password.templ",
		"views_auth_verify_email.tmpl":        "views/auth/verify_email.templ",
		"views_auth_resend_verification.tmpl": "views/auth/resend_verification.templ",

		"router_routes_auth.tmpl": "router/routes/auth.go",
	}
}
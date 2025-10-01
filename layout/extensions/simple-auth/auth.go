// Package simpleauth is an extension that provides functionality to integrate authentication features into a Go web application with no email validation.
package simpleauth

import (
	"fmt"
	"path/filepath"
	"time"
)

type TemplateData struct {
	ProjectName          string
	ModuleName           string
	Database             string
	SessionKey           string
	SessionEncryptionKey string
	TokenSigningKey      string
	PasswordSalt         string
	WithSimpleAuth       bool
}

type ProcessTemplateFunc func(targetDir, templateFile, targetPath string, data TemplateData) error

func ProcessAuthRecipe(
	targetDir string,
	data TemplateData,
	processTemplate ProcessTemplateFunc,
) error {
	mappings := getTemplateMappings(data.Database)

	for tmplFile, targetPath := range mappings {
		fullTmplPath := filepath.Join("simple-auth", "templates", tmplFile)
		if err := processTemplate(targetDir, fullTmplPath, targetPath, data); err != nil {
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
		"migration_001_users" + migrationSuffix: fmt.Sprintf(
			"database/migrations/%v_users.sql",
			time.Now().Format("20060102150405"),
		),

		"queries_users.tmpl": "database/queries/users.sql",

		"models_user.tmpl": "models/user.go",

		"middleware_auth.tmpl": "router/middleware/auth.go",

		"controllers_auth.tmpl": "controllers/auth.go",

		"views_auth_login.tmpl":  "views/auth/login.templ",
		"views_auth_signup.tmpl": "views/auth/signup.templ",

		"router_routes_auth.tmpl":  "router/routes/auth.go",
		"router_cookies_auth.tmpl": "router/cookies/auth.go",
	}
}

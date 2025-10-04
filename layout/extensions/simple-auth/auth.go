package simpleauth

import (
	"fmt"
	"os/exec"
	"path"
	"time"

	"github.com/mbvlabs/andurel/layout/extensions"
)

type Extension struct{}

const passwordValidationFunction = "func validatePasswordsMatch(sl validator.StructLevel) {\n" +
	"\tpwPair := sl.Current().Interface().(PasswordPair)\n\n" +
	"\tif pwPair.Password != pwPair.ConfirmPassword {\n" +
	"\t\tsl.ReportError(\n" +
	"\t\t\tpwPair.Password,\n" +
	"\t\t\t\"Password\",\n" +
	"\t\t\t\"Password\",\n" +
	"\t\t\t\"must match confirm password\",\n" +
	"\t\t\t\"\",\n" +
	"\t\t)\n" +
	"\t\tsl.ReportError(\n" +
	"\t\t\tpwPair.Password,\n" +
	"\t\t\t\"ConfirmPassword\",\n" +
	"\t\t\t\"ConfirmPassword\",\n" +
	"\t\t\t\"must match password\",\n" +
	"\t\t\t\"\",\n" +
	"\t\t)\n" +
	"\t}\n" +
	"}"

func (Extension) Name() string {
	return "simple-auth"
}

func (Extension) Apply(ctx *extensions.Context) error {
	if ctx == nil {
		return fmt.Errorf("simple-auth: context is nil")
	}

	if ctx.ProcessTemplate == nil {
		return fmt.Errorf("simple-auth: process template callback is nil")
	}

	if ctx.Data == nil {
		return fmt.Errorf("simple-auth: template data is nil")
	}

	if err := addSlotContributions(ctx); err != nil {
		return err
	}

	mappings := getTemplateMappings(ctx.Data.DatabaseDialect())

	for tmplFile, targetPath := range mappings {
		fullTmplPath := path.Join("simple-auth", "templates", tmplFile)
		if err := ctx.ProcessTemplate(fullTmplPath, targetPath, ctx.Data); err != nil {
			return fmt.Errorf("failed to process auth template %s: %w", tmplFile, err)
		}
	}

	if ctx.AddPostStep != nil {
		ctx.AddPostStep(func() error {
			cmd := exec.Command("go", "tool", "sqlc", "generate", "-f", "database/sqlc.yaml")
			cmd.Dir = ctx.TargetDir

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("sqlc generate failed: %w", err)
			}

			return nil
		})
	}

	return nil
}

func addSlotContributions(ctx *extensions.Context) error {
	slotSnippets := map[string][]string{
		"controllers:structFields": {
			"Auth   Auth",
		},
		"controllers:newArgs": {
			"cfg config.Config",
		},
		"controllers:newSetup": {
			"auth := newAuth(cfg, db)",
		},
		"controllers:newReturn": {
			"auth",
		},
		"cmd/app:setupArgs": {
			"cfg config.Config",
		},
		"cmd/app:controllerArgs": {
			"cfg",
		},
		"cmd/app:setupCallArgs": {
			"cfg",
		},
		"models:validatorRegistrations": {
			"v.RegisterStructValidation(validatePasswordsMatch, PasswordPair{})",
		},
		"routes:build": {
			"r = append(r, authRoutes...)",
		},
	}

	for slot, snippets := range slotSnippets {
		for _, snippet := range snippets {
			if err := ctx.AddSlotSnippet(slot, snippet); err != nil {
				return fmt.Errorf("simple-auth: failed to add snippet for %s: %w", slot, err)
			}
		}
	}

	if err := ctx.AddSlotSnippet("models:functions", passwordValidationFunction); err != nil {
		return fmt.Errorf("simple-auth: failed to add password validator: %w", err)
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

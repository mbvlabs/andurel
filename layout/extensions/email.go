package extensions

import (
	"fmt"
)

type Email struct{}

func (e Email) Name() string {
	return "email"
}

func (e Email) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("email: context or data is nil")
	}

	builder := ctx.Builder()

	moduleName := ctx.Data.GetModuleName()

	builder.AddMainImport(fmt.Sprintf("%s/email", moduleName))

	builder.AddControllerImport(fmt.Sprintf("%s/email", moduleName))

	builder.AddMainInitialization(
		"emailClient",
		"email.New(cfg)",
		"cfg",
	)

	builder.AddControllerDependency("emailClient", "email.Client")

	builder.AddConfigField("Email", "email")

	builder.AddTool("github.com/mailhog/MailHog")

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("email: failed to render templates: %w", err)
	}

	return nil
}

func (e Email) Dependencies() []string {
	return nil
}

func (e Email) renderTemplates(ctx *Context) error {
	templates := map[string]string{
		"email_email.tmpl":       "email/email.go",
		"email_base_layout.tmpl": "email/base_layout.templ",
		"email_components.tmpl":  "email/components.templ",
		"clients_mail_hog.tmpl":  "clients/mail_hog.go",
		"config_email.tmpl":      "config/email.go",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/email/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}

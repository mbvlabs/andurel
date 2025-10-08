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

	// Add email package import to main.go
	builder.AddMainImport(fmt.Sprintf("%s/email", moduleName))

	// Add email package import to controllers
	builder.AddControllerImport(fmt.Sprintf("%s/email", moduleName))

	// Add email service initialization in main.go
	builder.AddMainInitialization(
		"emailClient",
		"email.New()",
		"cfg",
	)

	// Add email sender as controller dependency
	builder.AddControllerDependency("emailClient", "email.Client")

	builder.AddConfigField("MailHogHost", "MAIL_HOG_HOST")
	builder.AddConfigField("MailHogPort", "MAIL_HOG_PORT")

	// Render all template files
	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("email: failed to render templates: %w", err)
	}

	return nil
}

func (e Email) renderTemplates(ctx *Context) error {
	templates := map[string]string{
		"email_email.tmpl":       "email/email.go",
		"email_base_layout.tmpl": "email/base_layout.templ",
		"email_components.tmpl":  "email/components.templ",
		"clients_mail_hog.tmpl":  "clients/mail_hog.go",
	}

	// Process each template
	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/email/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}

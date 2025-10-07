package email

import (
	"fmt"

	"github.com/mbvlabs/andurel/layout/extensions"
)

type Extension struct{}

func (e Extension) Name() string {
	return "email"
}

func (e Extension) Apply(ctx *extensions.Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("email: context or data is nil")
	}

	builder := ctx.Builder()
	if builder == nil {
		return fmt.Errorf("email: builder is nil")
	}

	moduleName := ctx.Data.GetModuleName()

	// Add email package import to main.go
	builder.AddMainImport(fmt.Sprintf("%s/email", moduleName))

	// Add email package import to controllers
	builder.AddControllerImport(fmt.Sprintf("%s/email", moduleName))

	// Add email service initialization in main.go
	builder.AddMainInitialization(
		"emailSender",
		"email.NewMailHog()",
		"cfg", // depends on config
	)

	// Add email sender as controller dependency
	builder.AddControllerDependency("emailSender", "email.Sender")

	// Render all template files
	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("email: failed to render templates: %w", err)
	}

	return nil
}

func (e Extension) renderTemplates(ctx *extensions.Context) error {
	templates := map[string]string{
		"email_email.tmpl":    "email/email.go",
		"email_mail_hog.tmpl": "email/mail_hog.go",
	}

	// Process each template
	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("email/templates/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}

func Register() error {
	return extensions.Register(Extension{})
}

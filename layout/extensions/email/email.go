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
		return fmt.Errorf("simple-auth: context or data is nil")
	}

	builder := ctx.Builder()
	if builder == nil {
		return fmt.Errorf("simple-auth: builder is nil")
	}

	moduleName := ctx.Data.GetModuleName()

	// Add config import - needed for the cfg config.Config dependency parameter
	builder.AddControllerImport(fmt.Sprintf("%s/config", moduleName))

	// Add config dependency to controller constructor
	builder.AddControllerDependency("cfg", "config.Config")
	builder.AddControllerDependency("email", "email.Sender")

	// Render all template files
	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("simple-auth: failed to render templates: %w", err)
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

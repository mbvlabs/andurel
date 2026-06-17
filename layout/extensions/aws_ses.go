package extensions

import "fmt"

type AwsSes struct{}

func (e AwsSes) Name() string {
	return "aws-ses"
}

func (e AwsSes) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("aws-ses: context or data is nil")
	}

	builder := ctx.Builder()

	// Add config field
	builder.AddConfigField("AwsSes", "awsSes")

	// Add env vars
	builder.AddEnvVar("AWS_REGION", "AwsSes", "us-east-1")
	builder.AddEnvVar("AWS_SES_ACCESS_KEY_ID", "AwsSes", "")
	builder.AddEnvVar("AWS_SES_SECRET_ACCESS_KEY", "AwsSes", "")
	builder.AddEnvVar("AWS_SES_CONFIGURATION_SET", "AwsSes", "")

	// Add email client service provide with env-based dispatch
	builder.AddServiceProvide(`func(cfg config.Config) (email.TransactionalSender, email.MarketingSender) {
	if config.Env == "production" {
		return mailclients.NewAwsSes(cfg), mailclients.NewAwsSes(cfg)
	}
	return mailclients.NewMailpit(cfg), mailclients.NewMailpit(cfg)
}`)

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("aws-ses: failed to render templates: %w", err)
	}

	return nil
}

func (e AwsSes) Dependencies() []string {
	return nil
}

func (e AwsSes) renderTemplates(ctx *Context) error {
	templates := map[string]string{
		"clients_email_aws_ses.tmpl": "clients/email/aws_ses.go",
		"config_aws_ses.tmpl":        "config/aws_ses.go",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/aws-ses/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}

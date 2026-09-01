package extensions

import "fmt"

// AwsSes adds AWS SES email client support to a scaffolded project.
type AwsSes struct{}

// Name returns the extension name used in lock files and CLI flags.
func (e AwsSes) Name() string {
	return "aws-ses"
}

// Apply adds AWS SES configuration, providers, and client files.
func (e AwsSes) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("aws-ses: context or data is nil")
	}

	builder := ctx.Builder()
	// Add config field
	builder.AddConfigField("AwsSes", "AwsSesCfg")

	builder.AddServiceProvide(
		`func(appCfg config.AppCfg, awsSesCfg config.AwsSesCfg, emailCfg config.EmailCfg) (email.TransactionalSender, email.MarketingSender) {
	if appCfg.GetEnvironment() == server.ProdEnvironment {
		return mailclients.NewAwsSes(awsSesCfg), mailclients.NewAwsSes(awsSesCfg)
	}

	client, err := email.NewMailpit(email.WithMailpitConfig(emailCfg.MailpitConfig()))
	if err != nil {
		panic(err)
	}

	return client, client
}`,
	)

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("aws-ses: failed to render templates: %w", err)
	}

	return nil
}

// Dependencies returns extension names that must be applied first.
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

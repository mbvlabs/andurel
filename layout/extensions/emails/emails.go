package emails

import (
	"fmt"
	"path"

	"github.com/mbvlabs/andurel/layout/extensions"
)

const envSlot = "env:variables"

type Extension struct{}

var templateMappings = map[string]string{
	"email_sender.tmpl":                  "emails/sender.go",
	"email_mail_hog.tmpl":                "emails/mail_hog.go",
	"email_mail_hog_test.tmpl":           "emails/mail_hog_test.go",
	"email_aws_ses.tmpl":                 "emails/aws_ses.go",
	"email_aws_ses_test.tmpl":            "emails/aws_ses_test.go",
	"email_email.tmpl":                   "emails/email.go",
	"email_email_test.tmpl":              "emails/email_test.go",
	"templates_transactional.templ.tmpl": "emails/templates/transactional.templ",
	"templates_marketing.templ.tmpl":     "emails/templates/marketing.templ",
}

func (Extension) Name() string {
	return "emails"
}

func (Extension) Apply(ctx *extensions.Context) error {
	if ctx == nil {
		return fmt.Errorf("emails: context is nil")
	}

	if ctx.ProcessTemplate == nil {
		return fmt.Errorf("emails: process template callback is nil")
	}

	if ctx.Data == nil {
		return fmt.Errorf("emails: template data is nil")
	}

	if err := addEnvVariables(ctx); err != nil {
		return err
	}

	for tmplFile, targetPath := range templateMappings {
		fullTmplPath := path.Join("emails", "templates", tmplFile)
		if err := ctx.ProcessTemplate(fullTmplPath, targetPath, ctx.Data); err != nil {
			return fmt.Errorf("emails: failed to process template %s: %w", tmplFile, err)
		}
	}

	return nil
}

func addEnvVariables(ctx *extensions.Context) error {
	lines := []string{
		"# Email configuration",
		"EMAIL_PROVIDER=mailhog",
		"MAILHOG_SMTP_ADDR=localhost:1025",
		"MAILHOG_FROM_EMAIL=${DEFAULT_SENDER_SIGNATURE}",
		"AWS_SES_REGION=us-east-1",
		"AWS_SES_SOURCE_EMAIL=${DEFAULT_SENDER_SIGNATURE}",
		"AWS_SES_CONFIGURATION_SET=",
	}

	for _, line := range lines {
		if err := ctx.AddSlotSnippet(envSlot, line); err != nil {
			return fmt.Errorf("emails: failed to contribute env variable: %w", err)
		}
	}

	return nil
}

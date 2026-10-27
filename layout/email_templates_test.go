package layout

import "testing"

func TestGeneratedMailSupportsSMTPAndRejectsMailpitInProduction(t *testing.T) {
	assertTemplateContains(t, "config_email.tmpl",
		"SMTPDriver",
		"LogDriver",
		"DefaultSMTPPort",
		"DefaultSMTPTLS",
		`e.String("SMTP_HOST", "")`,
		`e.String("SMTP_PORT", DefaultSMTPPort)`,
		`e.String("SMTP_TLS", DefaultSMTPTLS)`,
	)

	assertTemplateContains(t, "runtime_runtime.tmpl",
		"func validateEmailConfig(cfg config.Mail, app config.App) error",
		"if app.IsProduction() {",
		"set EMAIL_PROVIDER=%s",
		"case config.SMTPDriver, config.LogDriver:",
		"email.NewLog(slog.Default())",
	)
}

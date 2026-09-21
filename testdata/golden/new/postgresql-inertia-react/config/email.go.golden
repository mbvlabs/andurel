package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/pkg/email"
)

type MailDriver string

const (
	MailpitDriver MailDriver = "mailpit"

	DefaultMailpitHost = "0.0.0.0"
	DefaultMailpitPort = "1025"
)

type Mail struct {
	DefaultSenderSignature string
}

type MailTransport struct {
	Driver  MailDriver
	Mailpit email.MailpitConfig
}

func NewMail(app App) (Mail, error) {
	env := newEnvironment()
	cfg := Mail{
		DefaultSenderSignature: env.String("DEFAULT_SENDER_SIGNATURE", "noreply@"+app.Domain),
	}
	if strings.TrimSpace(cfg.DefaultSenderSignature) == "" {
		return Mail{}, fmt.Errorf("config: mail: DEFAULT_SENDER_SIGNATURE must not be empty")
	}

	return cfg, nil
}

func NewMailTransport(app App) (MailTransport, error) {
	env := newEnvironment()
	defaultDriver := MailDriver("")
	if !app.IsProduction() {
		defaultDriver = MailpitDriver
	}
	cfg := MailTransport{
		Driver: MailDriver(env.String("EMAIL_PROVIDER", string(defaultDriver))),
	}
	var driverErr error
	switch cfg.Driver {
	case MailpitDriver:
		cfg.Mailpit = email.MailpitConfig{
			Host: env.String("MAILPIT_HOST", DefaultMailpitHost),
			Port: env.String("MAILPIT_PORT", DefaultMailpitPort),
		}
		driverErr = cfg.Mailpit.Validate()
	default:
		driverErr = fmt.Errorf(
			"choose an installed EMAIL_PROVIDER in config/email.go (got %q)",
			cfg.Driver,
		)
	}

	if err := errors.Join(env.Err(), driverErr); err != nil {
		return MailTransport{}, fmt.Errorf("config: mail transport: %w", err)
	}

	return cfg, nil
}

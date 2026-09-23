package config

import (
	"fmt"
	"strings"
)

type MailDriver string

const (
	MailpitDriver MailDriver = "mailpit"

	DefaultMailpitHost = "0.0.0.0"
	DefaultMailpitPort = "1025"
)

type Mail struct {
	DefaultSenderSignature string
	Driver                 MailDriver
	Host                   string
	Port                   string
}

func NewMail(app App) (Mail, error) {
	env := newEnvironment()
	cfg := Mail{
		DefaultSenderSignature: env.String("DEFAULT_SENDER_SIGNATURE", "noreply@"+app.Domain),
		Driver:                 MailDriver(env.String("EMAIL_PROVIDER", string(MailpitDriver))),
		Host:                   env.String("MAILPIT_HOST", DefaultMailpitHost),
		Port:                   env.String("MAILPIT_PORT", DefaultMailpitPort),
	}
	if strings.TrimSpace(cfg.DefaultSenderSignature) == "" {
		return Mail{}, fmt.Errorf("config: mail: DEFAULT_SENDER_SIGNATURE must not be empty")
	}

	return cfg, nil
}

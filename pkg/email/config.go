package email

import (
	"fmt"
	"strings"
)

const (
	DefaultMailpitHost = "0.0.0.0"
	DefaultMailpitPort = "1025"
)

// MailpitConfig configures the development Mailpit SMTP client.
type MailpitConfig struct {
	Host string
	Port string
}

// DefaultMailpitConfig returns development-safe Mailpit defaults.
func DefaultMailpitConfig() MailpitConfig {
	return MailpitConfig{
		Host: DefaultMailpitHost,
		Port: DefaultMailpitPort,
	}
}

// Validate verifies Mailpit configuration.
func (config MailpitConfig) Validate() error {
	if strings.TrimSpace(config.Host) == "" {
		return fmt.Errorf("email: mailpit host cannot be empty")
	}
	if strings.TrimSpace(config.Port) == "" {
		return fmt.Errorf("email: mailpit port cannot be empty")
	}
	return nil
}

package email

import (
	"fmt"
	"strings"
)

// MailpitConfig configures a Mailpit SMTP client. Applications supply values;
// this type does not define defaults.
type MailpitConfig struct {
	Host string
	Port string
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

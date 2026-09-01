package email

type mailpitOptions struct {
	config MailpitConfig
}

// MailpitOption configures a Mailpit client.
type MailpitOption func(*mailpitOptions) error

// WithMailpitConfig applies Mailpit connection settings.
func WithMailpitConfig(config MailpitConfig) MailpitOption {
	return func(options *mailpitOptions) error {
		if err := config.Validate(); err != nil {
			return err
		}
		options.config = config
		return nil
	}
}

package email

import (
	"context"
	"fmt"
	"net"
)

var (
	_ TransactionalSender = (*Mailpit)(nil)
	_ MarketingSender     = (*Mailpit)(nil)
)

// Mailpit sends email through a development Mailpit SMTP server.
type Mailpit struct {
	host string
	port string
}

// NewMailpit constructs a Mailpit client from the supplied configuration.
func NewMailpit(config MailpitConfig) (*Mailpit, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("email: create mailpit client: %w", err)
	}

	return &Mailpit{
		host: config.Host,
		port: config.Port,
	}, nil
}

func (client *Mailpit) SendTransactional(
	ctx context.Context,
	payload TransactionalPayload,
) error {
	outbound := outboundMessage{
		From:        payload.From,
		To:          nonEmpty(payload.To),
		Cc:          payload.Cc,
		Bcc:         payload.Bcc,
		ReplyTo:     payload.ReplyTo,
		Subject:     payload.Subject,
		HTMLBody:    payload.HTMLBody,
		TextBody:    payload.TextBody,
		Attachments: payload.Attachments,
	}

	message, err := buildMessage(outbound)
	if err != nil {
		return err
	}

	return client.send(ctx, payload.From, outbound.recipients(), message)
}

func (client *Mailpit) SendMarketing(
	ctx context.Context,
	payload MarketingPayload,
) error {
	outbound := outboundMessage{
		From:     payload.From,
		To:       payload.To,
		ReplyTo:  payload.ReplyTo,
		Subject:  payload.Subject,
		HTMLBody: payload.HTMLBody,
		TextBody: payload.TextBody,
	}

	message, err := buildMessage(outbound)
	if err != nil {
		return err
	}

	return client.send(ctx, payload.From, outbound.recipients(), message)
}

func (client *Mailpit) send(
	ctx context.Context,
	from string,
	recipients []string,
	message []byte,
) error {
	return sendMail(ctx, sendOptions{
		address:    net.JoinHostPort(client.host, client.port),
		host:       client.host,
		tlsMode:    SMTPTLSNone,
		from:       from,
		recipients: recipients,
		message:    message,
	})
}

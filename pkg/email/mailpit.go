package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
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

// NewMailpit constructs a Mailpit client from the supplied options.
func NewMailpit(options ...MailpitOption) (*Mailpit, error) {
	var settings mailpitOptions
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&settings); err != nil {
			return nil, fmt.Errorf("email: create mailpit client: %w", err)
		}
	}
	if err := settings.config.Validate(); err != nil {
		return nil, fmt.Errorf("email: create mailpit client: %w", err)
	}

	return &Mailpit{
		host: settings.config.Host,
		port: settings.config.Port,
	}, nil
}

func (client *Mailpit) SendTransactional(
	ctx context.Context,
	payload TransactionalPayload,
) error {
	addr := fmt.Sprintf("%s:%s", client.host, client.port)

	boundary := "boundary-mailpit-client"
	headers := make(map[string]string)
	headers["From"] = payload.From
	headers["To"] = payload.To

	if len(payload.Cc) > 0 {
		headers["Cc"] = strings.Join(payload.Cc, ", ")
	}

	if len(payload.Bcc) > 0 {
		headers["Bcc"] = strings.Join(payload.Bcc, ", ")
	}

	if payload.ReplyTo != "" {
		headers["Reply-To"] = payload.ReplyTo
	}

	headers["Subject"] = payload.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = fmt.Sprintf(
		"multipart/alternative; boundary=\"%s\"",
		boundary,
	)

	var message strings.Builder
	for key, value := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	message.WriteString("\r\n")

	if payload.TextBody != "" {
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		message.WriteString("\r\n")
		message.WriteString(payload.TextBody)
		message.WriteString("\r\n")
	}

	if payload.HTMLBody != "" {
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		message.WriteString("\r\n")
		message.WriteString(payload.HTMLBody)
		message.WriteString("\r\n")
	}

	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	recipients := []string{payload.To}
	recipients = append(recipients, payload.Cc...)
	recipients = append(recipients, payload.Bcc...)

	return smtp.SendMail(
		addr,
		nil,
		payload.From,
		recipients,
		[]byte(message.String()),
	)
}

func (client *Mailpit) SendMarketing(
	ctx context.Context,
	payload MarketingPayload,
) error {
	addr := fmt.Sprintf("%s:%s", client.host, client.port)

	boundary := "boundary-mailpit-client"
	headers := make(map[string]string)
	headers["From"] = payload.From

	if len(payload.To) > 0 {
		headers["To"] = strings.Join(payload.To, ", ")
	}

	if payload.ReplyTo != "" {
		headers["Reply-To"] = payload.ReplyTo
	}

	headers["Subject"] = payload.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = fmt.Sprintf(
		"multipart/alternative; boundary=\"%s\"",
		boundary,
	)

	var message strings.Builder
	for key, value := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	message.WriteString("\r\n")

	if payload.TextBody != "" {
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		message.WriteString("\r\n")
		message.WriteString(payload.TextBody)
		message.WriteString("\r\n")
	}

	if payload.HTMLBody != "" {
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		message.WriteString("\r\n")
		message.WriteString(payload.HTMLBody)
		message.WriteString("\r\n")
	}

	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return smtp.SendMail(
		addr,
		nil,
		payload.From,
		payload.To,
		[]byte(message.String()),
	)
}

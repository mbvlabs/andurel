package email

import "fmt"

// Transformer renders an email template to both of its bodies.
type Transformer interface {
	ToHTML() (string, error)
	ToText() (string, error)
}

// Compose renders t into transactional data addressed to a single recipient.
func Compose(t Transformer, to, from, subject string) (TransactionalData, error) {
	html, err := t.ToHTML()
	if err != nil {
		return TransactionalData{}, fmt.Errorf("email: render HTML body: %w", err)
	}

	text, err := t.ToText()
	if err != nil {
		return TransactionalData{}, fmt.Errorf("email: render text body: %w", err)
	}

	return TransactionalData{
		To:       to,
		From:     from,
		Subject:  subject,
		HTMLBody: html,
		TextBody: text,
	}, nil
}

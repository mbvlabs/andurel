package email

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

type Transformer interface {
	ToHTML() (string, error)
	ToText() (string, error)
}

var (
	ErrUnsubscribeURLRequired = errors.New("marketing emails require unsubscribe URL")
	ErrMissingRecipient       = errors.New("missing recipient email address")
	ErrMissingSender          = errors.New("missing sender email address")
	ErrMissingSubject         = errors.New("missing email subject")
	ErrMissingHTMLBody        = errors.New("missing email HTML body")
)

type ValidationError struct {
	Err error
}

func (e ValidationError) Error() string {
	return e.Err.Error()
}

func (e ValidationError) Unwrap() error {
	return e.Err
}

type TemporaryError struct {
	Err error
}

func (e TemporaryError) Error() string {
	return e.Err.Error()
}

func (e TemporaryError) Unwrap() error {
	return e.Err
}

type PermanentError struct {
	Err error
}

func (e PermanentError) Error() string {
	return e.Err.Error()
}

func (e PermanentError) Unwrap() error {
	return e.Err
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return true
	}

	return errors.Is(err, ErrUnsubscribeURLRequired) ||
		errors.Is(err, ErrMissingRecipient) ||
		errors.Is(err, ErrMissingSender) ||
		errors.Is(err, ErrMissingSubject) ||
		errors.Is(err, ErrMissingHTMLBody)
}

func IsRetryable(err error) bool {
	var tempErr TemporaryError
	if errors.As(err, &tempErr) {
		return true
	}

	var permErr PermanentError
	if errors.As(err, &permErr) {
		return false
	}

	if IsValidationError(err) {
		return false
	}

	return true
}

type TransactionalData struct {
	To          string
	Cc          []string
	Bcc         []string
	From        string
	ReplyTo     string
	Subject     string
	HTMLBody    string
	TextBody    string
	Attachments []Attachment
	Metadata    map[string]string
}

type MarketingData struct {
	To               []string
	From             string
	ReplyTo          string
	Subject          string
	HTMLBody         string
	TextBody         string
	UnsubscribeURL   string
	UnsubscribeGroup string
	Tags             []string
	Metadata         map[string]string
	TrackOpens       bool
	TrackClicks      bool
}

type Attachment struct {
	Name        string
	Content     []byte
	ContentType string
	Inline      bool
}

type TransactionalPayload struct {
	To          string
	Cc          []string
	Bcc         []string
	From        string
	ReplyTo     string
	Subject     string
	HTMLBody    string
	TextBody    string
	Attachments []Attachment
	Metadata    map[string]string
}

type MarketingPayload struct {
	To               []string
	From             string
	ReplyTo          string
	Subject          string
	HTMLBody         string
	TextBody         string
	UnsubscribeURL   string
	UnsubscribeGroup string
	Tags             []string
	Metadata         map[string]string
	TrackOpens       bool
	TrackClicks      bool
}

type TransactionalSender interface {
	SendTransactional(ctx context.Context, payload TransactionalPayload) error
}

type MarketingSender interface {
	SendMarketing(ctx context.Context, payload MarketingPayload) error
}

func SendTransactional(
	ctx context.Context,
	data TransactionalData,
	sender TransactionalSender,
) error {
	if data.To == "" && len(data.Cc) == 0 && len(data.Bcc) == 0 {
		return ValidationError{Err: ErrMissingRecipient}
	}

	if data.From == "" {
		return ValidationError{Err: ErrMissingSender}
	}

	if data.Subject == "" {
		return ValidationError{Err: ErrMissingSubject}
	}

	if data.HTMLBody == "" {
		return ValidationError{Err: ErrMissingHTMLBody}
	}

	payload := TransactionalPayload{
		To:          data.To,
		Cc:          data.Cc,
		Bcc:         data.Bcc,
		From:        data.From,
		ReplyTo:     data.ReplyTo,
		Subject:     data.Subject,
		HTMLBody:    data.HTMLBody,
		TextBody:    data.TextBody,
		Attachments: data.Attachments,
		Metadata:    data.Metadata,
	}

	return sender.SendTransactional(ctx, payload)
}

func SendMarketing(ctx context.Context, data MarketingData, sender MarketingSender) error {
	if data.UnsubscribeURL == "" {
		return ErrUnsubscribeURLRequired
	}

	if data.HTMLBody == "" {
		return ValidationError{Err: ErrMissingHTMLBody}
	}

	payload := MarketingPayload{
		To:               data.To,
		From:             data.From,
		ReplyTo:          data.ReplyTo,
		Subject:          data.Subject,
		HTMLBody:         data.HTMLBody,
		TextBody:         data.TextBody,
		UnsubscribeURL:   data.UnsubscribeURL,
		UnsubscribeGroup: data.UnsubscribeGroup,
		Tags:             data.Tags,
		Metadata:         data.Metadata,
		TrackOpens:       data.TrackOpens,
		TrackClicks:      data.TrackClicks,
	}

	return sender.SendMarketing(ctx, payload)
}

func renderComponent(component templ.Component) (string, error) {
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func HTMLToText(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var result strings.Builder
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "style", "script", "head":
				return
			case "a":
				var linkText strings.Builder
				var extractLinkText func(*html.Node)
				extractLinkText = func(node *html.Node) {
					if node.Type == html.TextNode {
						linkText.WriteString(node.Data)
					}
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						extractLinkText(child)
					}
				}
				extractLinkText(n)

				var href string
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						href = attr.Val
						break
					}
				}

				text := strings.TrimSpace(linkText.String())
				if text != "" {
					result.WriteString(text)
				}

				if href != "" && href != text {
					result.WriteString(" (")
					result.WriteString(href)
					result.WriteString(")")
				}
				result.WriteString(" ")
				return
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6":
				result.WriteString("\n")
			case "tr", "li":
				result.WriteString("\n")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6":
				result.WriteString("\n")
			}
		}
	}

	extract(doc)

	text := result.String()
	text = strings.TrimSpace(text)

	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return text, nil
}

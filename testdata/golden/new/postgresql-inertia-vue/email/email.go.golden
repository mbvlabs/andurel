package email

import pkgemail "github.com/mbvlabs/andurel/pkg/email"

// Transformer converts application-owned email templates into HTML and text.
type Transformer interface {
	ToHTML() (string, error)
	ToText() (string, error)
}

// HTMLToText converts rendered HTML email content into a plain-text body.
func HTMLToText(htmlContent string) (string, error) {
	return pkgemail.HTMLToText(htmlContent)
}

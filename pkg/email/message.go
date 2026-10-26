package email

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

// outboundMessage is the provider-neutral form of an email. Transports turn a
// payload into one of these and hand it to buildMessage.
type outboundMessage struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	ReplyTo     string
	Subject     string
	HTMLBody    string
	TextBody    string
	Attachments []Attachment
}

// recipients returns the envelope recipients: everyone who receives a copy,
// including Bcc recipients, who never appear in the message headers.
func (message outboundMessage) recipients() []string {
	recipients := make([]string, 0, len(message.To)+len(message.Cc)+len(message.Bcc))
	recipients = append(recipients, message.To...)
	recipients = append(recipients, message.Cc...)
	recipients = append(recipients, message.Bcc...)

	return recipients
}

type messageHeader struct {
	name  string
	value string
}

// buildMessage renders a multipart message. A message without attachments is a
// multipart/alternative document; attachments wrap that document in a
// multipart/mixed one. The Bcc header is deliberately absent; Bcc addresses
// travel only in the SMTP envelope.
func buildMessage(message outboundMessage) ([]byte, error) {
	if err := validateMessage(message); err != nil {
		return nil, err
	}

	alternativeBoundary, err := randomToken(16)
	if err != nil {
		return nil, fmt.Errorf("email: generate MIME boundary: %w", err)
	}

	messageID, err := randomToken(16)
	if err != nil {
		return nil, fmt.Errorf("email: generate Message-ID: %w", err)
	}

	var alternative strings.Builder
	if message.TextBody != "" {
		writeBodyPart(&alternative, alternativeBoundary, "text/plain", message.TextBody)
	}

	if message.HTMLBody != "" {
		writeBodyPart(&alternative, alternativeBoundary, "text/html", message.HTMLBody)
	}

	alternative.WriteString(fmt.Sprintf("--%s--\r\n", alternativeBoundary))

	now := time.Now()

	var body strings.Builder
	headers := []messageHeader{
		{name: "From", value: formatAddressList(message.From)},
		{name: "Subject", value: mime.QEncoding.Encode("utf-8", message.Subject)},
		{name: "Date", value: now.Format(time.RFC1123Z)},
		{
			name:  "Message-ID",
			value: fmt.Sprintf("<%d.%s@%s>", now.UnixNano(), messageID, messageIDDomain(message.From)),
		},
		{name: "MIME-Version", value: "1.0"},
	}

	if to := formatAddressList(strings.Join(message.To, ", ")); to != "" {
		headers = append(headers, messageHeader{name: "To", value: to})
	}

	if cc := formatAddressList(strings.Join(message.Cc, ", ")); cc != "" {
		headers = append(headers, messageHeader{name: "Cc", value: cc})
	}

	if message.ReplyTo != "" {
		headers = append(headers, messageHeader{name: "Reply-To", value: formatAddressList(message.ReplyTo)})
	}

	if len(message.Attachments) == 0 {
		headers = append(headers, messageHeader{
			name:  "Content-Type",
			value: fmt.Sprintf("multipart/alternative; boundary=\"%s\"", alternativeBoundary),
		})
	} else {
		mixedBoundary, err := randomToken(16)
		if err != nil {
			return nil, fmt.Errorf("email: generate MIME boundary: %w", err)
		}

		headers = append(headers, messageHeader{
			name:  "Content-Type",
			value: fmt.Sprintf("multipart/mixed; boundary=\"%s\"", mixedBoundary),
		})

		for _, header := range headers {
			if err := writeHeader(&body, header.name, header.value); err != nil {
				return nil, err
			}
		}
		body.WriteString("\r\n")

		body.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
		body.WriteString(fmt.Sprintf(
			"Content-Type: multipart/alternative; boundary=\"%s\"\r\n",
			alternativeBoundary,
		))
		body.WriteString("\r\n")
		body.WriteString(alternative.String())

		for _, attachment := range message.Attachments {
			if err := writeAttachmentPart(&body, mixedBoundary, attachment); err != nil {
				return nil, err
			}
		}

		body.WriteString(fmt.Sprintf("--%s--\r\n", mixedBoundary))

		return []byte(body.String()), nil
	}

	for _, header := range headers {
		if err := writeHeader(&body, header.name, header.value); err != nil {
			return nil, err
		}
	}
	body.WriteString("\r\n")
	body.WriteString(alternative.String())

	return []byte(body.String()), nil
}

// validateMessage rejects malformed addresses and header values that could
// break out of their header line. Both are caller mistakes, not transient
// delivery failures.
func validateMessage(message outboundMessage) error {
	values := []struct {
		name  string
		value string
	}{
		{name: "From", value: message.From},
		{name: "Reply-To", value: message.ReplyTo},
		{name: "Subject", value: message.Subject},
	}

	for _, value := range values {
		if err := validateHeaderValue(value.name, value.value); err != nil {
			return err
		}
	}

	if _, err := mail.ParseAddress(message.From); err != nil {
		return ValidationError{Err: fmt.Errorf("email: invalid From address: %w", err)}
	}

	lists := []struct {
		name   string
		values []string
	}{
		{name: "To", values: message.To},
		{name: "Cc", values: message.Cc},
		{name: "Bcc", values: message.Bcc},
	}

	for _, list := range lists {
		if len(list.values) == 0 {
			continue
		}

		if _, err := mail.ParseAddressList(strings.Join(list.values, ", ")); err != nil {
			return ValidationError{Err: fmt.Errorf("email: invalid %s address: %w", list.name, err)}
		}
	}

	if message.ReplyTo != "" {
		if _, err := mail.ParseAddressList(message.ReplyTo); err != nil {
			return ValidationError{Err: fmt.Errorf("email: invalid Reply-To address: %w", err)}
		}
	}

	for _, attachment := range message.Attachments {
		if err := validateHeaderValue("attachment name", attachment.Name); err != nil {
			return err
		}
		if err := validateHeaderValue("attachment content type", attachment.ContentType); err != nil {
			return err
		}
	}

	return nil
}

func validateHeaderValue(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return ValidationError{Err: fmt.Errorf("email: %s header contains a newline", name)}
	}

	return nil
}

func writeHeader(body *strings.Builder, name, value string) error {
	if err := validateHeaderValue(name, value); err != nil {
		return err
	}

	body.WriteString(name + ": " + value + "\r\n")

	return nil
}

func writeBodyPart(body *strings.Builder, boundary, contentType, content string) {
	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString(fmt.Sprintf("Content-Type: %s; charset=\"UTF-8\"\r\n", contentType))
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	body.WriteString("\r\n")

	writer := quotedprintable.NewWriter(body)
	_, _ = writer.Write([]byte(content))
	_ = writer.Close()
	body.WriteString("\r\n")
}

func writeAttachmentPart(body *strings.Builder, boundary string, attachment Attachment) error {
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	typeParams := map[string]string{}
	dispositionParams := map[string]string{}
	if attachment.Name != "" {
		typeParams["name"] = attachment.Name
		dispositionParams["filename"] = attachment.Name
	}

	formattedType := mime.FormatMediaType(contentType, typeParams)
	if formattedType == "" {
		formattedType = "application/octet-stream"
	}

	disposition := "attachment"
	if attachment.Inline {
		disposition = "inline"
	}

	formattedDisposition := mime.FormatMediaType(disposition, dispositionParams)
	if formattedDisposition == "" {
		formattedDisposition = disposition
	}

	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString("Content-Type: " + formattedType + "\r\n")
	body.WriteString("Content-Transfer-Encoding: base64\r\n")
	body.WriteString("Content-Disposition: " + formattedDisposition + "\r\n")
	body.WriteString("\r\n")

	encoder := base64.NewEncoder(base64.StdEncoding, body)
	if _, err := encoder.Write(attachment.Content); err != nil {
		return fmt.Errorf("email: encode attachment %q: %w", attachment.Name, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("email: encode attachment %q: %w", attachment.Name, err)
	}
	body.WriteString("\r\n")

	return nil
}

// formatAddressList re-encodes the display names in an address list with RFC
// 2047 so non-ASCII names survive the trip. Unparseable input is passed through
// unchanged; buildMessage validates addresses before it calls this.
func formatAddressList(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return value
	}

	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.Name == "" {
			formatted = append(formatted, address.Address)
			continue
		}

		formatted = append(formatted, fmt.Sprintf(
			"%s <%s>",
			mime.QEncoding.Encode("utf-8", address.Name),
			address.Address,
		))
	}

	return strings.Join(formatted, ", ")
}

// messageIDDomain returns the domain of the From address, falling back to
// localhost when it cannot be parsed.
func messageIDDomain(from string) string {
	address, err := mail.ParseAddress(from)
	if err != nil {
		return "localhost"
	}

	at := strings.LastIndex(address.Address, "@")
	if at < 0 || at == len(address.Address)-1 {
		return "localhost"
	}

	return address.Address[at+1:]
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return hex.EncodeToString(buffer), nil
}

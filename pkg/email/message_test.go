package email

import (
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
)

func TestBuildMessageRejectsInvalidAddressesAndHeaderInjection(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*outboundMessage)
	}{
		{
			name: "newline in From",
			mutate: func(message *outboundMessage) {
				message.From = "noreply@example.com\r\nBcc: hidden@example.com"
			},
		},
		{
			name: "newline in To",
			mutate: func(message *outboundMessage) {
				message.To = []string{"person@example.com\r\nBcc: hidden@example.com"}
			},
		},
		{
			name: "newline in Cc",
			mutate: func(message *outboundMessage) {
				message.Cc = []string{"person@example.com\nBcc: hidden@example.com"}
			},
		},
		{
			name: "newline in Reply-To",
			mutate: func(message *outboundMessage) {
				message.ReplyTo = "noreply@example.com\nBcc: hidden@example.com"
			},
		},
		{
			name: "newline in Subject",
			mutate: func(message *outboundMessage) {
				message.Subject = "Hello\r\nBcc: hidden@example.com"
			},
		},
		{
			name: "newline in attachment name",
			mutate: func(message *outboundMessage) {
				message.Attachments = []Attachment{{
					Name:    "report.pdf\r\nBcc: hidden@example.com",
					Content: []byte("content"),
				}}
			},
		},
		{
			name: "newline in attachment content type",
			mutate: func(message *outboundMessage) {
				message.Attachments = []Attachment{{
					Name:        "report.pdf",
					Content:     []byte("content"),
					ContentType: "application/pdf\nBcc: hidden@example.com",
				}}
			},
		},
		{
			name: "invalid From address",
			mutate: func(message *outboundMessage) {
				message.From = "not an address"
			},
		},
		{
			name: "invalid To address",
			mutate: func(message *outboundMessage) {
				message.To = []string{"not an address"}
			},
		},
		{
			name: "invalid Cc address",
			mutate: func(message *outboundMessage) {
				message.Cc = []string{"not an address"}
			},
		},
		{
			name: "invalid Bcc address",
			mutate: func(message *outboundMessage) {
				message.Bcc = []string{"not an address"}
			},
		},
		{
			name: "invalid Reply-To address",
			mutate: func(message *outboundMessage) {
				message.ReplyTo = "not an address"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := validOutboundMessage()
			test.mutate(&message)

			raw, err := buildMessage(message)
			if err == nil {
				t.Fatalf("buildMessage() = %d bytes, want validation error", len(raw))
			}
			if _, ok := errors.AsType[ValidationError](err); !ok {
				t.Fatalf("error = %T, want ValidationError", err)
			}
			if !IsValidationError(err) {
				t.Fatalf("IsValidationError(%v) = false, want true", err)
			}
		})
	}
}

func TestBuildMessageEncodesBodiesAsQuotedPrintable(t *testing.T) {
	suffix := " Héllo wörld"
	htmlBody := strings.Repeat("A", 5000-len([]rune(suffix))) + suffix

	raw, err := buildMessage(outboundMessage{
		From:     "noreply@example.com",
		To:       []string{"person@example.com"},
		Subject:  "Hello",
		HTMLBody: htmlBody,
	})
	if err != nil {
		t.Fatalf("buildMessage() = %v, want nil error", err)
	}

	message, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/alternative" {
		t.Fatalf("content type = %q, want multipart/alternative", mediaType)
	}

	part, err := multipart.NewReader(message.Body, params["boundary"]).NextRawPart()
	if err != nil {
		t.Fatalf("next part: %v", err)
	}

	if got := part.Header.Get("Content-Transfer-Encoding"); got != "quoted-printable" {
		t.Fatalf("Content-Transfer-Encoding = %q, want quoted-printable", got)
	}

	encoded, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("read part: %v", err)
	}

	for line := range strings.SplitSeq(string(encoded), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if len(line) > 76 {
			t.Fatalf("encoded line is %d chars, want at most 76: %q", len(line), line)
		}
	}

	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(encoded))))
	if err != nil {
		t.Fatalf("decode quoted-printable body: %v", err)
	}
	if string(decoded) != htmlBody {
		t.Fatalf("decoded body does not round-trip: got %d bytes, want %d", len(decoded), len(htmlBody))
	}
}

func TestBuildMessageWrapsAttachmentsInMultipartMixed(t *testing.T) {
	attachments := []Attachment{
		{
			Name:        "report.pdf",
			Content:     []byte("%PDF-1.4 attachment content"),
			ContentType: "application/pdf",
		},
		{
			Name:        "logo.png",
			Content:     []byte{0x89, 'P', 'N', 'G'},
			ContentType: "image/png",
			Inline:      true,
		},
	}

	raw, err := buildMessage(outboundMessage{
		From:        "noreply@example.com",
		To:          []string{"person@example.com"},
		Subject:     "Hello",
		HTMLBody:    "<p>Hello</p>",
		TextBody:    "Hello",
		Attachments: attachments,
	})
	if err != nil {
		t.Fatalf("buildMessage() = %v, want nil error", err)
	}

	message, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("content type = %q, want multipart/mixed", mediaType)
	}

	reader := multipart.NewReader(message.Body, params["boundary"])

	alternative, err := reader.NextRawPart()
	if err != nil {
		t.Fatalf("next alternative part: %v", err)
	}

	alternativeType, alternativeParams, err := mime.ParseMediaType(alternative.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse alternative content type: %v", err)
	}
	if alternativeType != "multipart/alternative" {
		t.Fatalf("alternative content type = %q, want multipart/alternative", alternativeType)
	}

	alternativeReader := multipart.NewReader(alternative, alternativeParams["boundary"])
	for _, want := range []string{"text/plain", "text/html"} {
		part, err := alternativeReader.NextRawPart()
		if err != nil {
			t.Fatalf("next %s part: %v", want, err)
		}

		partType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse %s content type: %v", want, err)
		}
		if partType != want {
			t.Fatalf("alternative part type = %q, want %s", partType, want)
		}
		if got := part.Header.Get("Content-Transfer-Encoding"); got != "quoted-printable" {
			t.Fatalf("%s Content-Transfer-Encoding = %q, want quoted-printable", want, got)
		}
	}

	for _, attachment := range attachments {
		part, err := reader.NextRawPart()
		if err != nil {
			t.Fatalf("next attachment part: %v", err)
		}

		if got := part.Header.Get("Content-Transfer-Encoding"); got != "base64" {
			t.Fatalf("attachment Content-Transfer-Encoding = %q, want base64", got)
		}

		disposition, dispositionParams, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if err != nil {
			t.Fatalf("parse attachment disposition: %v", err)
		}

		wantDisposition := "attachment"
		if attachment.Inline {
			wantDisposition = "inline"
		}
		if disposition != wantDisposition {
			t.Fatalf("attachment disposition = %q, want %s", disposition, wantDisposition)
		}
		if dispositionParams["filename"] != attachment.Name {
			t.Fatalf("attachment filename = %q, want %q", dispositionParams["filename"], attachment.Name)
		}

		contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse attachment content type: %v", err)
		}
		if contentType != attachment.ContentType {
			t.Fatalf("attachment content type = %q, want %q", contentType, attachment.ContentType)
		}

		encoded, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read attachment: %v", err)
		}
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(string(encoded))))
		if err != nil {
			t.Fatalf("decode attachment: %v", err)
		}
		if string(decoded) != string(attachment.Content) {
			t.Fatalf("attachment content = %q, want %q", decoded, attachment.Content)
		}
	}
}

func validOutboundMessage() outboundMessage {
	return outboundMessage{
		From:     "noreply@example.com",
		To:       []string{"person@example.com"},
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	}
}

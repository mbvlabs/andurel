package email

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogSenderLogsTransactionalMessage(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	sender := NewLog(slog.New(slog.NewTextHandler(&output, nil)))

	err := sender.SendTransactional(context.Background(), TransactionalPayload{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Verify your email",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	})
	if err != nil {
		t.Fatalf("SendTransactional: %v", err)
	}

	logged := output.String()
	for _, want := range []string{
		"EMAIL_PROVIDER=log",
		"person@example.com",
		"Verify your email",
	} {
		if !strings.Contains(logged, want) {
			t.Fatalf("log missing %q: %s", want, logged)
		}
	}
	if strings.Contains(logged, "<p>Hello</p>") {
		t.Fatalf("log must not contain the message body: %s", logged)
	}
}

func TestLogSenderLogsMarketingMessage(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	sender := NewLog(slog.New(slog.NewTextHandler(&output, nil)))

	err := sender.SendMarketing(context.Background(), MarketingPayload{
		To:             []string{"one@example.com", "two@example.com"},
		From:           "news@example.com",
		Subject:        "Release notes",
		HTMLBody:       "<p>News</p>",
		UnsubscribeURL: "https://example.com/unsubscribe",
	})
	if err != nil {
		t.Fatalf("SendMarketing: %v", err)
	}

	if logged := output.String(); !strings.Contains(logged, "Release notes") {
		t.Fatalf("log = %s", logged)
	}
}

func TestNewLogDefaultsToSlogDefault(t *testing.T) {
	t.Parallel()

	if sender := NewLog(nil); sender.logger == nil {
		t.Fatal("expected a logger")
	}
}

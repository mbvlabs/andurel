package email

import (
	"context"
	"log/slog"
)

var (
	_ TransactionalSender = (*Log)(nil)
	_ MarketingSender     = (*Log)(nil)
)

// Log logs messages instead of delivering them. It makes production-like local
// runs and smoke tests possible without an SMTP server or Mailpit, and it is
// allowed in every environment because nothing leaves the process.
type Log struct {
	logger *slog.Logger
}

// NewLog constructs a sender that logs messages. A nil logger uses
// slog.Default.
func NewLog(logger *slog.Logger) *Log {
	if logger == nil {
		logger = slog.Default()
	}

	return &Log{logger: logger}
}

func (sender *Log) SendTransactional(
	ctx context.Context,
	payload TransactionalPayload,
) error {
	sender.logger.InfoContext(ctx, "email: transactional message logged (EMAIL_PROVIDER=log)",
		slog.String("to", payload.To),
		slog.Int("cc", len(payload.Cc)),
		slog.Int("bcc", len(payload.Bcc)),
		slog.String("from", payload.From),
		slog.String("subject", payload.Subject),
		slog.Int("html_bytes", len(payload.HTMLBody)),
		slog.Int("text_bytes", len(payload.TextBody)),
		slog.Int("attachments", len(payload.Attachments)),
	)

	return nil
}

func (sender *Log) SendMarketing(
	ctx context.Context,
	payload MarketingPayload,
) error {
	sender.logger.InfoContext(ctx, "email: marketing message logged (EMAIL_PROVIDER=log)",
		slog.Any("to", payload.To),
		slog.String("from", payload.From),
		slog.String("subject", payload.Subject),
		slog.Int("html_bytes", len(payload.HTMLBody)),
		slog.Int("text_bytes", len(payload.TextBody)),
	)

	return nil
}

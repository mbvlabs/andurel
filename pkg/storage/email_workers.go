package storage

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/email"
	"github.com/mbvlabs/andurel/pkg/telemetry"
	"github.com/riverqueue/river"
)

// SendTransactionalEmailArgs carries one transactional email to its worker.
type SendTransactionalEmailArgs struct {
	Data email.TransactionalData
}

func (SendTransactionalEmailArgs) Kind() string { return "send_transactional_email" }

// SendMarketingEmailArgs carries one marketing email to its worker.
type SendMarketingEmailArgs struct {
	Data email.MarketingData
}

func (SendMarketingEmailArgs) Kind() string { return "send_marketing_email" }

// AddEmailWorkers registers the framework email workers on workers.
func AddEmailWorkers(
	workers *river.Workers,
	transactional email.TransactionalSender,
	marketing email.MarketingSender,
) {
	river.AddWorker(workers, NewSendTransactionalEmailWorker(transactional))
	river.AddWorker(workers, NewSendMarketingEmailWorker(marketing))
}

// SendTransactionalEmailWorker delivers a transactional email.
type SendTransactionalEmailWorker struct {
	river.WorkerDefaults[SendTransactionalEmailArgs]
	sender email.TransactionalSender
}

func NewSendTransactionalEmailWorker(
	sender email.TransactionalSender,
) *SendTransactionalEmailWorker {
	return &SendTransactionalEmailWorker{
		sender: sender,
	}
}

func (w *SendTransactionalEmailWorker) Work(
	ctx context.Context,
	job *river.Job[SendTransactionalEmailArgs],
) error {
	// The recipient address stays out of the span: traces are not a place
	// for personal data.
	ctx, span := telemetry.Start(ctx, "queue.send_transactional_email")
	defer span.End()

	return SendTransactional(ctx, job.Args.Data, w.sender)
}

// SendTransactional delivers data, cancelling the job on errors a retry cannot
// fix.
func SendTransactional(
	ctx context.Context,
	data email.TransactionalData,
	sender email.TransactionalSender,
) error {
	err := email.SendTransactional(ctx, data, sender)
	if err != nil {
		telemetry.Error(ctx, "send transactional email failed", "error", err)
		if !email.IsRetryable(err) {
			return river.JobCancel(telemetry.Fail(ctx, err))
		}

		return telemetry.Fail(ctx, err)
	}

	telemetry.Info(ctx, "transactional email sent")
	return nil
}

// SendMarketingEmailWorker delivers a marketing email.
type SendMarketingEmailWorker struct {
	river.WorkerDefaults[SendMarketingEmailArgs]
	sender email.MarketingSender
}

func NewSendMarketingEmailWorker(sender email.MarketingSender) *SendMarketingEmailWorker {
	return &SendMarketingEmailWorker{
		sender: sender,
	}
}

func (w *SendMarketingEmailWorker) Work(
	ctx context.Context,
	job *river.Job[SendMarketingEmailArgs],
) error {
	err := email.SendMarketing(ctx, job.Args.Data, w.sender)
	if err != nil {
		if !email.IsRetryable(err) {
			return river.JobCancel(err)
		}

		return err
	}

	return nil
}

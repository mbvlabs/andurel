package storage

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/postmand"
	"github.com/mbvlabs/andurel/pkg/telemetry"
	"github.com/riverqueue/river"
)

// SendTransactionalEmailArgs carries one transactional email to its worker.
type SendTransactionalEmailArgs struct {
	Data postmand.TransactionalData
}

func (SendTransactionalEmailArgs) Kind() string { return "send_transactional_email" }

// SendMarketingEmailArgs carries one marketing email to its worker.
type SendMarketingEmailArgs struct {
	Data postmand.MarketingData
}

func (SendMarketingEmailArgs) Kind() string { return "send_marketing_email" }

// AddEmailWorkers registers the framework email workers on workers.
func AddEmailWorkers(
	workers *river.Workers,
	transactional postmand.TransactionalSender,
	marketing postmand.MarketingSender,
) {
	river.AddWorker(workers, NewSendTransactionalEmailWorker(transactional))
	river.AddWorker(workers, NewSendMarketingEmailWorker(marketing))
}

// SendTransactionalEmailWorker delivers a transactional email.
type SendTransactionalEmailWorker struct {
	river.WorkerDefaults[SendTransactionalEmailArgs]
	sender postmand.TransactionalSender
}

func NewSendTransactionalEmailWorker(
	sender postmand.TransactionalSender,
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
	data postmand.TransactionalData,
	sender postmand.TransactionalSender,
) error {
	err := postmand.SendTransactional(ctx, data, sender)
	if err != nil {
		telemetry.Error(ctx, "send transactional email failed", "error", err)
		if !postmand.IsRetryable(err) {
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
	sender postmand.MarketingSender
}

func NewSendMarketingEmailWorker(sender postmand.MarketingSender) *SendMarketingEmailWorker {
	return &SendMarketingEmailWorker{
		sender: sender,
	}
}

func (w *SendMarketingEmailWorker) Work(
	ctx context.Context,
	job *river.Job[SendMarketingEmailArgs],
) error {
	err := postmand.SendMarketing(ctx, job.Args.Data, w.sender)
	if err != nil {
		if !postmand.IsRetryable(err) {
			return river.JobCancel(err)
		}

		return err
	}

	return nil
}

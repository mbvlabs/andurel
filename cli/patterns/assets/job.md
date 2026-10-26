# Job

A job is a River worker: typed args, a `Work` method, and registration with
the queue processor. Jobs coordinate with domains and services; they do not
hold business rules of their own.

## Files

### queue/jobs/<name>.go
```go
package jobs

import (
	"context"

	"github.com/riverqueue/river"
)

// Mailer is the slice of the email sender this worker needs; the queue package
// wires the real transactional sender.
type Mailer interface {
	SendWelcome(ctx context.Context, userID string) error
}

type SendWelcomeEmailArgs struct {
	UserID string `json:"user_id"`
}

func (SendWelcomeEmailArgs) Kind() string { return "send_welcome_email" }

type SendWelcomeEmailWorker struct {
	river.WorkerDefaults[SendWelcomeEmailArgs]
	mailer Mailer
}

func NewSendWelcomeEmailWorker(mailer Mailer) *SendWelcomeEmailWorker {
	return &SendWelcomeEmailWorker{mailer: mailer}
}

func (w *SendWelcomeEmailWorker) Work(ctx context.Context, job *river.Job[SendWelcomeEmailArgs]) error {
	return w.mailer.SendWelcome(ctx, job.Args.UserID)
}
```

## Rules

- Args are a plain struct with `json` tags and a `Kind()` string.
- Register the worker explicitly in `queue/workers.go` (`NewWorkers`); there is no reflection.
- The framework email workers and their `storage.SendTransactionalEmailArgs` / `storage.SendMarketingEmailArgs` types live in `pkg/storage`; `storage.AddEmailWorkers` registers them alongside your own.
- Enqueue through `storage.InsertQueue` from a service, not from a handler. When the job depends on data written in the same use case, enqueue on `i.queue.InTx(tx)` inside `storage.AtomicOps` so the job is only visible to workers after the data commits.
- Keep `Work` idempotent; River may retry.
- No `domains/internal/queries` import.

## Checklist

- [ ] Args struct with `json` tags and a stable `Kind()`.
- [ ] Worker registered in `queue/workers.go`.
- [ ] Enqueue call made from a service (on a bound queue when it must commit with data).
- [ ] `andurel inspect jobs --json` lists the new file.

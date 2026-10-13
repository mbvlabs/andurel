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
- Enqueue through `storage.InsertQueue` from a service, not from a handler.
- Keep `Work` idempotent; River may retry.
- No `domains/internal/queries` import.

## Checklist

- [ ] Args struct with `json` tags and a stable `Kind()`.
- [ ] Worker registered in `queue/workers.go`.
- [ ] Enqueue call made from a service.
- [ ] `andurel inspect jobs --json` lists the new file.

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

## Periodic jobs

A recurring job reuses the same args and worker types; River inserts it on a
schedule instead of a service. Return one `*river.PeriodicJob` per schedule
from `queue/PeriodicJobs()`; `runtime.NewQueue` passes the list to
`storage.WithRiverPeriodicJobs`, so no other wiring is needed.

```go
// queue/queue.go
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.DeleteExpiredTokensArgs{}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: true},
		),
	}
}
```

```go
// queue/jobs/delete_expired_tokens.go
package jobs

// DeleteExpiredTokensArgs has no fields: the worker deletes every token whose
// expiry has passed.
type DeleteExpiredTokensArgs struct{}

func (DeleteExpiredTokensArgs) Kind() string { return "delete_expired_tokens" }
```

```go
// queue/delete_expired_tokens.go
func (w *DeleteExpiredTokensWorker) Work(
	ctx context.Context,
	_ *river.Job[jobs.DeleteExpiredTokensArgs],
) error {
	ctx, span := telemetry.Start(ctx, "queue.delete_expired_tokens")
	defer span.End()

	deleted, err := w.tokens.DestroyExpired(ctx, time.Now())
	if err != nil {
		return telemetry.Fail(ctx, err)
	}

	telemetry.Info(ctx, "deleted expired tokens", "tokens.deleted", deleted)

	return nil
}
```

The constructor function must not block and receives no arguments; returning
`nil` args skips the insertion for that tick. `RunOnStart: true` makes a new
leader run housekeeping immediately instead of waiting a full interval.

## Checklist

- [ ] Args struct with `json` tags and a stable `Kind()`.
- [ ] Worker registered in `queue/workers.go`.
- [ ] Enqueue call made from a service (on a bound queue when it must commit with data).
- [ ] Recurring jobs returned from `queue/PeriodicJobs()`.
- [ ] `andurel inspect jobs --json` lists the args type as a job and the worker type as a worker.

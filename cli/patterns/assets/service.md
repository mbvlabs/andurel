# Service

A service holds application logic: use cases that span several domains, the
queue, email, or config. It answers "what should happen across entities?"
rather than "what is true about this entity?". It is not a pass-through
wrapper around a single domain method.

A service takes its dependencies individually: a `storage.Transactor`, the
domain receivers it uses, and the queue. A use case whose writes must commit
together calls `storage.AtomicOps` and, at the top of the callback, binds the
receivers it needs to the transaction with `InTx(tx)`. Enqueueing on a bound
queue makes the job commit with the data.

## Files

### services/<name>.go
```go
package services

import (
	"context"
	"fmt"

	"app/config"
	"app/domains"
	"app/queue/jobs"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Identity struct {
	transactor storage.Transactor
	users      domains.Users
	tokens     domains.Tokens
	queue      storage.InsertQueue
	pepper     string
}

func NewIdentity(
	transactor storage.Transactor,
	users domains.Users,
	tokens domains.Tokens,
	queue storage.InsertQueue,
	authCfg config.Auth,
	mailCfg config.Mail,
) Identity {
	return Identity{transactor: transactor, users: users, tokens: tokens, queue: queue, pepper: authCfg.Pepper}
}

func (i Identity) RegisterUser(ctx context.Context, data RegisterUserData) (domains.User, error) {
	var user domains.User

	err := storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {
		users, queue := i.users.InTx(tx), i.queue.InTx(tx)

		created, err := users.Create(ctx, i.pepper, domains.CreateUserData{
			Email: data.Email,
			PasswordPair: domains.PasswordPair{
				Password:        data.Password,
				ConfirmPassword: data.ConfirmPassword,
			},
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		// Job args persist in river_job: enqueue the id, and let the worker
		// mint the code via i.VerificationEmail and render the email.
		if _, err := queue.Insert(ctx, jobs.SendVerificationEmailArgs{UserID: created.ID}, nil); err != nil {
			return fmt.Errorf("queue verification email: %w", err)
		}

		user = created
		return nil
	})
	if err != nil {
		return domains.User{}, err
	}

	return user, nil
}
```

## Commit and rollback

`storage.AtomicOps` is the only thing that commits or rolls back; `storage.Tx`
has no `Commit` or `Rollback`. The callback's return value decides:

- Return `nil` → commit. `AtomicOps` returns the commit error, if any.
- Return an error → roll back. `AtomicOps` returns that error unchanged, so `errors.Is` works.
- Panic → roll back, then re-panic.
- Commit but still report a failure (e.g. persist a failed-attempt counter): return `nil`, set a local flag, and map the flag after `AtomicOps` returns.
- When `AtomicOps` runs on a bound transactor (a receiver's own atomic method called on an `InTx` receiver), it joins the caller's transaction and leaves commit and rollback to the owner.
- Side effects outside Postgres (HTTP calls, cache writes) never roll back. Do them after `AtomicOps` returns `nil`, or enqueue them on the bound queue so they only run after commit.

## Rules

- A service takes `storage.Transactor`, its domain receivers, and `storage.InsertQueue` individually, plus config. It never names `storage.Connection`/`storage.Transaction`, never calls `storage.Unwrap`, and never imports `domains/internal/queries`.
- Writes that must commit together run in `storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error { … })`.
- Bind at the top of the callback (`users, queue := i.users.InTx(tx), i.queue.InTx(tx)`) and use only the bound locals inside it. `i.users` inside the callback runs outside the transaction; `andurel verify` flags it (`tx-escape`).
- Helpers take their dependencies as parameters (`tokens domains.Tokens`, `queue storage.InsertQueue`) rather than reaching for `i.tokens`/`i.queue`.
- Public use cases do not call each other inside a callback; a nested public call opens an independent transaction. Share logic through unexported helpers that take bound dependencies (`andurel verify` flags this as `tx-public-call`).
- Job args are stored in `river_job` for the retention period. Never enqueue secrets (verification codes, reset tokens or URLs, emails rendered with them): enqueue ids, and have the worker call a service method that mints the secret and renders the email at send time.
- Assign results to locals declared before `AtomicOps`, and only on the success path; return the zero value on error.
- Do not re-validate entity rules; the domain's `Create`/`Update` do. Only check input specific to the use case (e.g. a login form's required fields).
- Keep constructors explicit (`NewIdentity(...)`); the runtime wires them.

## Checklist

- [ ] Constructor takes `storage.Transactor`, each receiver, and the queue explicitly.
- [ ] Multi-write use cases run inside `storage.AtomicOps`, binding with `InTx(tx)` at the top and using only bound locals.
- [ ] Jobs that depend on the written data are enqueued on `i.queue.InTx(tx)`.
- [ ] Job args hold ids, never codes, tokens, or emails rendered with them.
- [ ] No import of `domains/internal/queries`; no `storage.Connection`, `storage.Transaction`, or `storage.Unwrap`.
- [ ] The service adds orchestration rather than forwarding one call.
- [ ] Wired once in `internal/runtime`.

# Domain transactions: opaque `storage.Tx` + `storage.AtomicOps`

- **Status:** Draft for review, rev 4
- **Date:** 2026-10-15
- **Follows:** `plans/rfc-domain-transactions-options.md` (Option 1 chosen)
- **Relates to:** `plans/rfc-agent-first-go-native.md` §5.4, §6, §8.3, §10, §12

## Decisions from review

| # | Feedback | Decision |
| --- | --- | --- |
| 1 | Don't like `RunValue` | **Removed.** Value-producing use cases assign to a local declared before the call. |
| 2 | One function, named `AtomicOps` | **`storage.AtomicOps(ctx, transactor, fn)`** is the only public way to run a transaction. |
| 3 | Show the preference for atomicity inside domains | §5. |
| 4 | No savepoints | **Dropped.** |
| 5 | Naming question unclear | **Names fixed:** `storage.Transactor`, `storage.Tx`, `storage.AtomicOps`. |
| 6 | `In` → `InTx` | **Done.** |
| 7 | The per-service binder struct breaks easily | **Removed.** |
| 8 | Why do services need `InTx`? → chose A | **Services have no `InTx`.** Each use case binds, inside its callback, exactly the receivers it uses (§4). |
| 9 | What about rollbacks and commits? | **`AtomicOps` is the only thing that commits or rolls back.** The return value of the callback decides; `Tx` has no `Commit`/`Rollback`. Full semantics in §2.1. |

---

## 1. The idea

`pkg/storage` gains a `Transactor` and an opaque `Tx`: an open transaction with **no query methods**. `storage.AtomicOps` opens a transaction and hands the callback the `Tx`.

Three things can bind to a `Tx` with `InTx(tx)`:

- **domain receivers** (`users.InTx(tx)`)
- **the queue** (`queue.InTx(tx)`), so jobs commit with the data
- **the `Transactor`** itself, so a domain method that is atomic on its own joins a caller's transaction instead of opening a second one

Services are plain structs of receivers. A use case that must commit several operations together calls `AtomicOps` and binds what it needs at the top of the callback.

---

## 2. `pkg/storage` API

```go
// pkg/storage/tx.go
package storage

// Tx is an open transaction. It has no query methods: holding one lets a
// caller bind to the transaction (InTx) but not issue SQL. Only AtomicOps
// creates a usable Tx.
type Tx struct {
	tx Transaction
}

// Transactor opens transactions for AtomicOps. One from NewTransactor opens a
// new transaction per call; one returned by InTx joins tx instead.
type Transactor struct {
	conn  Connection
	bound Tx
}

func NewTransactor(conn Connection) Transactor {
	return Transactor{conn: conn}
}

// InTx returns a Transactor whose AtomicOps joins tx instead of opening a
// new transaction.
func (t Transactor) InTx(tx Tx) Transactor {
	return Transactor{bound: tx}
}

// AtomicOps runs fn inside one transaction. With an unbound Transactor it
// opens the transaction and commits when fn returns nil; any error or panic
// rolls back. With a bound Transactor it runs fn in the bound transaction
// and leaves commit and rollback to the transaction's owner.
func AtomicOps(ctx context.Context, t Transactor, fn func(ctx context.Context, tx Tx) error) error {
	if t.bound.tx != nil {
		return fn(ctx, t.bound)
	}
	return RunInTransaction(ctx, t.conn, func(ctx context.Context, tx Transaction) error {
		return fn(ctx, Tx{tx: tx})
	})
}

// Unwrap returns the underlying transaction so a domain receiver can bind
// its generated queries to it. andurel verify fails on any call outside
// domains/. It panics on a zero Tx, which only arises from Tx{} literals.
func Unwrap(tx Tx) Transaction {
	if tx.tx == nil {
		panic("storage: zero Tx; obtain one from AtomicOps")
	}
	return tx.tx
}
```

The queue gets the same `InTx` shape. This is a `pkg/` change, so it needs a changelog entry:

```go
// pkg/storage/queue.go
type InsertQueue interface {
	Insert(context.Context, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error)
	InsertMany(context.Context, []river.InsertManyParams) ([]*rivertype.JobInsertResult, error)
	// InTx returns a queue whose inserts join tx: jobs become visible to
	// workers only if tx commits.
	InTx(tx Tx) InsertQueue
}
```

A bound `QueueInsert` routes `Insert`/`InsertMany` to River's `InsertTx`/`InsertManyTx`. The explicit `InsertTx(ctx, tx, …)` methods are removed: whether an insert is transactional depends only on whether the queue was bound.

`Connection`, `Transaction` and `RunInTransaction` remain, but they are **wiring-only** (runtime, cmd, seeds, migrations) or **domain-only**, enforced by `verify` (§7).

### 2.1 Commit and rollback

The **callback's return value is the only control.** `Tx` deliberately has no `Commit` or `Rollback`, so no code can end a transaction it doesn't own, forget to end one, or commit half of a joined one.

| The callback… | Owner (`AtomicOps` on an unbound `Transactor`) | Joined (`AtomicOps` on a bound `Transactor`) |
| --- | --- | --- |
| returns `nil` | **commits**; `AtomicOps` returns the commit error, if any | does nothing; the owner decides later |
| returns an error | **rolls back**; `AtomicOps` returns the callback's error unchanged, so `errors.Is` works | returns the error to the owner's callback |
| panics | **rolls back**, then re-panics | the panic unwinds to the owner, which rolls back and re-panics |
| runs past `ctx` cancellation | the next query fails, the callback returns that error, and the transaction rolls back | same, via the owner |

How each case plays out in practice:

- **Roll back on purpose:** return an error. If the use case "fails" but there is nothing exceptional to report, return a sentinel the service maps (e.g. `ErrInvalidResetCode`); the data is rolled back and the caller gets the sentinel.
- **Commit, but report failure:** return `nil` and set a local flag, then map the flag after `AtomicOps` returns. `VerifyEmail` does this (§4.3) so the attempt count persists even when the code is wrong.
- **A joined callback's error:** if it came from a failed SQL statement, Postgres has already aborted the whole transaction. The owner cannot continue and must return an error (there are no savepoints). If it is a domain error raised *before* any failing statement (validation, `ErrNotFound`), the owner may handle it and continue.
- **A commit error is ambiguous:** a network failure during `COMMIT` can leave the transaction committed or not. `AtomicOps` returns it wrapped (`storage: commit transaction: …`). Callers treat it like any other error, and the work should be idempotent if it gets retried.
- **Side effects outside Postgres never roll back.** That covers HTTP calls, cache writes, and anything else not on the transaction. Do them after `AtomicOps` returns `nil`, or enqueue them through a bound queue so they only run after commit. That is exactly what `queue.InTx(tx)` is for.

Two fixes to `RunInTransaction` come with this, since `AtomicOps` builds on it:

```go
func RunInTransaction(ctx context.Context, conn Connection, fn func(context.Context, Transaction) error) (err error) {
	tx, err := conn.BeginTransaction(ctx)
	if err != nil {
		return fmt.Errorf("storage: begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		// Roll back even when ctx is already cancelled: a cancelled ctx would
		// make Rollback fail fast and leave the connection to be discarded.
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}() // runs on error returns and on panics; the panic keeps unwinding

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err // already wrapped as "storage: commit transaction: …"
	}
	committed = true
	return nil
}
```

- **Rollback uses `context.WithoutCancel(ctx)`.** Today `_ = tx.Rollback(ctx)` runs with the caller's ctx, which is often the cancelled one.
- **`BeginTransaction` errors are wrapped**, so they are distinguishable from callback errors.

Test coverage for this lives in `pkg/storage`:
- callback error → rolled back, same error returned
- panic → rolled back and re-panicked
- joined callback → no commit until the owner returns
- cancelled ctx → rolled back

---

## 3. Domain receivers

```go
// domains/user.go
package domains

// andurel:table users

type Users struct {
	queries *queries.Queries
}

func NewUsers(db storage.Connection) Users {
	return Users{queries: queries.New(db)}
}

// InTx returns a Users whose calls run inside tx.
func (u Users) InTx(tx storage.Tx) Users {
	return Users{queries: queries.New(storage.Unwrap(tx))}
}
```

A receiver with no multi-statement methods needs nothing more.

---

## 4. Services: bind inline, per use case

### 4.1 Shape

There is no `InTx`, no stored config, and no binder: the service holds its dependencies and nothing else.

```go
// services/identity.go
type Identity struct {
	transactor storage.Transactor
	users      domains.Users
	tokens     domains.Tokens
	queue      storage.InsertQueue

	pepper                 string
	previousPeppers        []string
	tokenSigningKey        string
	defaultSenderSignature string
}

func NewIdentity(
	transactor storage.Transactor,
	users domains.Users,
	tokens domains.Tokens,
	queue storage.InsertQueue,
	authCfg config.Auth,
	mailCfg config.Mail,
) Identity { /* … */ }
```

### 4.2 RegisterUser

```go
// RegisterUser creates the user, a verification code, and the verification
// email job in one transaction: all three commit or none do.
func (i Identity) RegisterUser(ctx context.Context, data RegisterUserData) (domains.User, error) {
	var user domains.User

	err := storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {
		users, tokens, queue := i.users.InTx(tx), i.tokens.InTx(tx), i.queue.InTx(tx)

		created, err := users.Create(ctx, i.pepper, domains.CreateUserData{
			Email:        data.Email,
			PasswordPair: domains.PasswordPair{Password: data.Password, ConfirmPassword: data.ConfirmPassword},
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		code, err := tokens.ReplaceCode(ctx, i.tokenSigningKey, userEmailVerification, created.ID /* … */)
		if err != nil {
			return err
		}

		if err := i.enqueueVerificationEmail(ctx, queue, created, code); err != nil {
			return err
		}

		user = created
		return nil
	})
	if err != nil {
		return domains.User{}, err
	}

	return user, nil
}

// enqueueVerificationEmail takes the queue to use. Pass a bound queue to
// enqueue inside a transaction.
func (i Identity) enqueueVerificationEmail(ctx context.Context, queue storage.InsertQueue, user domains.User, code string) error {
	html, text, err := renderVerifyEmail(code)
	if err != nil {
		return err
	}
	_, err = queue.Insert(ctx, jobs.SendTransactionalEmailArgs{ /* … */ }, nil)
	return err
}
```

The rules this shows:

- **Bind at the top of the callback.** Use only the bound locals (`users`, `tokens`, `queue`) inside it. `i.users` inside the callback is a `verify` failure (§7).
- **Helpers take their dependencies as parameters** (`queue storage.InsertQueue`, `tokens domains.Tokens`) rather than reaching for `i.queue`/`i.tokens`, so the caller decides whether they run in a transaction.
- **Assign the result only on the success path** (`user = created`). On error the caller returns the zero value, so a rolled-back partial value never escapes.

### 4.3 VerifyEmail: commit the attempt count when the code is wrong

```go
var (
	user    domains.User
	invalid bool
)

err := storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {
	users, tokens := i.users.InTx(tx), i.tokens.InTx(tx)

	token, err := tokens.FindByUserAndScope(ctx, data.UserID, userEmailVerification)
	if errors.Is(err, domains.ErrNotFound) {
		invalid = true
		return nil
	}
	// … IncrementAttempts / lockout / IsValid exactly as today, via tokens …
	// … users.Find / users.Update / tokens.Destroy …
	user = updated
	return nil
})
if err != nil {
	return domains.User{}, err
}
if invalid {
	return domains.User{}, invalidVerificationCode()
}
return user, nil
```

### 4.4 Reusing logic across use cases

Public use cases do **not** call each other inside a transaction: a nested public call would open its own, independent transaction. Logic shared by several use cases goes into an unexported helper that takes bound dependencies:

```go
func (i Identity) ResendEmailVerification(ctx context.Context, user domains.User) error {
	if user.HasValidatedEmail() {
		return nil
	}
	return storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {
		return i.sendVerificationCode(ctx, i.tokens.InTx(tx), i.queue.InTx(tx), user)
	})
}

// sendVerificationCode replaces the user's code and enqueues the email
// through the given (typically transaction-bound) dependencies.
func (i Identity) sendVerificationCode(ctx context.Context, tokens domains.Tokens, queue storage.InsertQueue, user domains.User) error {
	code, err := tokens.ReplaceCode(ctx, i.tokenSigningKey, userEmailVerification, user.ID /* … */)
	if err != nil {
		return err
	}
	return i.enqueueVerificationEmail(ctx, queue, user, code)
}
```

`RegisterUser` can then call `i.sendVerificationCode(ctx, tokens, queue, created)` with its own bound locals.

---

## 5. Atomicity inside one domain

A receiver with multi-statement methods holds a `Transactor` and rebinds it in `InTx`. Standalone, its method opens a transaction; on a bound receiver, the bound `Transactor` makes `AtomicOps` join the caller's transaction:

```go
// domains/token.go
type Tokens struct {
	transactor storage.Transactor
	queries    *queries.Queries
}

func NewTokens(db storage.Connection) Tokens {
	return Tokens{transactor: storage.NewTransactor(db), queries: queries.New(db)}
}

func (t Tokens) InTx(tx storage.Tx) Tokens {
	return Tokens{transactor: t.transactor.InTx(tx), queries: queries.New(storage.Unwrap(tx))}
}

// ReplaceCode destroys any outstanding code for (user, scope) and issues a
// new one, atomically.
func (t Tokens) ReplaceCode(ctx context.Context, signingKey, scope string, userID uuid.UUID /* … */) (string, error) {
	var code string

	err := storage.AtomicOps(ctx, t.transactor, func(ctx context.Context, tx storage.Tx) error {
		tokens := t.InTx(tx)

		if err := tokens.DestroyByUserAndScope(ctx, userID, scope); err != nil {
			return err
		}
		c, err := tokens.CreateCode(ctx, signingKey, scope /* … */)
		if err != nil {
			return err
		}
		code = c
		return nil
	})
	if err != nil {
		return "", err
	}

	return code, nil
}
```

`RegisterUser` calls `tokens.ReplaceCode` on a bound `Tokens`, so the destroy-and-create runs inside the registration transaction. Seeds or a standalone caller get their own transaction. `Users` stays `{queries}`: only receivers with multi-statement methods carry a `transactor`.

---

## 6. Wiring and tests

```go
// internal/runtime/runtime.go
transactor := storage.NewTransactor(db)
users := domains.NewUsers(db)
tokens := domains.NewTokens(db)

identity := services.NewIdentity(transactor, users, tokens, queueInsert, cfg.Auth, cfg.Mail)

handlers.NewAPIHandler(db).Register(router) // healthChecker; runtime may hold db
```

`App` goes back to `DB storage.Connection` and drops `Store`. Seeds use `domains.NewUsers(db)`. Adding a domain means a `NewX(db)` here plus a constructor parameter and field in each service that uses it.

```go
func TestIdentity_RegisterUserIsAtomic(t *testing.T) {
	t.Parallel()
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	identity := services.NewIdentity(
		storage.NewTransactor(db),
		domains.NewUsers(db),
		domains.NewTokens(db),
		failingQueue{}, // Insert returns an error; InTx returns itself
		testAuthConfig(), testMailConfig(),
	)

	_, err := identity.RegisterUser(t.Context(), validRegistration())
	if err == nil {
		t.Fatal("expected enqueue failure to fail registration")
	}

	_, err = domains.NewUsers(db).FindByEmail(t.Context(), validRegistration().Email)
	if !errors.Is(err, domains.ErrNotFound) {
		t.Fatalf("user must not exist after rollback, got %v", err)
	}
}
```

---

## 7. Enforcement (`andurel verify`)

| Rule | Fails when |
| --- | --- |
| `queries-import` | a file outside `domains/` imports `domains/internal/queries` (unchanged) |
| `db-handle` | a file outside `domains/`, `internal/runtime/`, `cmd/`, `seeds/` or `migrations/` names `storage.Connection`, `storage.Transaction` or `storage.RunInTransaction`, or imports pgx (`pgtype` allowed) (unchanged) |
| `tx-unwrap` | `storage.Unwrap` is called outside `domains/` |
| `tx-escape` | inside a func literal passed to `storage.AtomicOps`, the enclosing receiver's field is used (`i.users`, `t.queries`, …), unless the use is immediately `.InTx(tx)` or the field is a plain value (config strings and the like). It catches running a query outside the transaction by accident. |
| `tx-public-call` | inside a func literal passed to `storage.AtomicOps`, an exported method is called on the enclosing receiver (`i.ResendEmailVerification(…)`). This catches the nested independent transaction. |

All are syntactic. `tx-escape` reads field types from the receiver's struct declaration in the same package, and flags fields whose type has an `InTx` method.

---

## 8. Changes from the current branch state

- **Remove** `domains_store.tmpl` and `domains_jobs.tmpl`. Receivers get `NewX(db)` + `InTx(tx)`; `Tokens` gains `transactor` + `ReplaceCode`.
- **`pkg/storage`:** add `Transactor` (with `InTx`), `Tx`, `AtomicOps` and `Unwrap`. `InsertQueue` gains `InTx`, and its `*Tx` methods are removed. Changelog entry.
- **Services:** take `storage.Transactor`, receivers and the queue individually; use cases call `AtomicOps` and bind inline; shared helpers take bound dependencies as parameters.
- **Switch from `Store`:** seeds, runtime, the API handler, the patterns (`domain`, `service`, `composition`, `email`, `test-*`), the skills, the README, RFC §5.4/§6/§10/§18, and the fixtures.
- **`verify`:** replace `tx-bypass` with `tx-unwrap`, `tx-escape` and `tx-public-call`. The manifest's `store.go` edit becomes a runtime `NewX(db)` edit. In `inspect domains`, `wired` now means "constructed in runtime".
- **Kept as-is:** the validation consolidation, structured violations, the generated-drift step, exit code 9, `--entity` scoping, and the table annotation.

# Domain transactions: design options

- **Status:** Options for review (replaces the `domains.Store` prototype)
- **Date:** 2026-10-14
- **Relates to:** `plans/rfc-agent-first-go-native.md` §5.4, §6, §8.3

## Verdict on the review finding

> "This design is NOT correct. We shouldn't have jobs or domains being part of this."

**Confirmed: a design issue introduced by the current changes.** `layout/templates/domains_store.tmpl` (new and untracked) makes `Store` an aggregate of every receiver (`Users`, `Tokens`, `Jobs`). It also moves River enqueueing into `domains/jobs.go`. That causes three problems:

- **The Store becomes a dependency bag.** Every service and handler that takes `domains.Store` gets every domain, so a constructor no longer shows what it uses. This is a service locator under another name, which goes against §3.2 ("Explicit over injected") and §3.7.
- **`Jobs` puts queue infrastructure in the business-logic package.** River is in Postgres, but enqueueing is application plumbing, not "what is true about this entity".
- **The transaction mechanism and the domain registry are fused.** Adding a domain means editing the transaction type.

The goals the Store was serving still stand, and every option below must meet them:

| # | Requirement |
| --- | --- |
| R1 | Only `domains` can issue SQL. Nothing else holds anything with `Exec`/`Query`. |
| R2 | A service can run several domain calls in one transaction. |
| R3 | Transactional enqueue: a job commits with the data, with no jobs code in `domains`. |
| R4 | No package globals. Each test builds its own graph and can run with `t.Parallel()`. |
| R5 | Adding a domain changes nothing in the transaction mechanism. |
| R6 | Constructors list real dependencies (`users domains.Users`), not a bag. |

All options share one idea: **a wrapper in `pkg/storage` owns the transaction, and domains and the queue both join it.** They differ in how the transaction reaches a domain call.

---

## Option 1: Opaque transaction handle (`storage.Transactor` + `storage.Tx`)

A wrapper around the connection runs the transaction and hands the callback an **opaque** handle. The handle has no query methods: holding it lets you *join* the transaction, not issue SQL. Domain receivers rebind with `In(tx)`, and River joins with `InsertTx(ctx, tx, …)`.

```go
// pkg/storage/transactor.go
// Tx is an open transaction with no query methods. Pass it to domain
// receivers (users.In(tx)) or the queue (InsertTx); it cannot run SQL.
type Tx struct{ tx Transaction }

type Transactor struct{ conn Connection }

func NewTransactor(conn Connection) Transactor { return Transactor{conn: conn} }

// Run commits when fn returns nil and rolls back otherwise.
func (t Transactor) Run(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error

// Unwrap exposes the transaction to domains. andurel verify fails on any
// call outside domains/.
func Unwrap(tx Tx) Transaction
```

```go
// domains/user.go
func NewUsers(db storage.Connection) Users { return Users{queries: queries.New(db)} }
func (u Users) In(tx storage.Tx) Users     { return Users{queries: queries.New(storage.Unwrap(tx))} }
```

```go
// services/registration.go
type Identity struct {
	tx     storage.Transactor
	users  domains.Users
	tokens domains.Tokens
	queue  storage.InsertQueue
}

func (i Identity) RegisterUser(ctx context.Context, data RegisterUserData) (domains.User, error) {
	var user domains.User
	err := i.tx.Run(ctx, func(ctx context.Context, tx storage.Tx) error {
		created, err := i.users.In(tx).Create(ctx, i.pepper, data.toCreate())
		if err != nil {
			return err
		}
		code, err := i.tokens.In(tx).CreateCode(ctx /* … */)
		if err != nil {
			return err
		}
		_, err = i.queue.InsertTx(ctx, tx, verifyEmailArgs(created, code), nil)
		user = created
		return err
	})
	return user, err
}
```

- **R1:** `storage.Tx` has no methods; `storage.Unwrap` outside `domains/` is a `verify` failure. `Connection` stays in runtime, cmd and seeds only.
- **R3:** `InsertQueue.InsertTx` takes `storage.Tx`, so jobs stay in the service.
- **R5/R6:** adding a domain is just `NewX(db)` in runtime. Services list `domains.Users`, `domains.Tokens`, and so on.
- **Footgun:** calling `i.users.Create` inside `Run` without `.In(tx)` escapes the transaction. `verify` can flag a method call on a domain-typed field inside a `Run` callback whose chain doesn't go through `.In(`. The check is purely syntactic.
- **Nesting:** explicit. Pass `tx` down; a helper that receives a `tx` uses `.In(tx)`.
- **Cost:** a `storage.Tx` type, `Transactor`, `Unwrap`, and an `InsertTx` signature change (a `pkg/` change, so a changelog entry).

## Option 2: Context-scoped transaction, resolved by domain receivers

Same `Transactor` wrapper, but `Run` puts the transaction on `ctx` and the callback receives only the new `ctx`. Each receiver resolves its executor per call.

```go
// pkg/storage
func (t Transactor) Run(ctx context.Context, fn func(ctx context.Context) error) error // joins if ctx already carries a tx
func TxFrom(ctx context.Context) (Transaction, bool)
```

```go
// domains/user.go
func (u Users) q(ctx context.Context) *queries.Queries {
	if tx, ok := storage.TxFrom(ctx); ok {
		return queries.New(tx)
	}
	return u.queries
}

func (u Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
	entity, err := u.q(ctx).GetUser[User](ctx, id)
	// …
}
```

```go
// services/registration.go
err := i.tx.Run(ctx, func(ctx context.Context) error {
	created, err := i.users.Create(ctx, i.pepper, data.toCreate())
	// …
	_, err = i.queue.Insert(ctx, args, nil) // QueueInsert sees the tx on ctx and uses InsertTx
	return err
})
```

- **R1:** services never see a transaction value at all; `storage.TxFrom` outside `domains/` and `pkg/` is a `verify` failure.
- **R3:** automatic. `QueueInsert.Insert` checks `ctx` and calls `InsertTx`.
- **Nesting:** free. An inner `Run` joins, so domain methods can open their own `Run` for internal atomicity and still compose.
- **Footgun:** using the *outer* `ctx` inside the callback silently escapes the transaction. The variables shadow each other, but it is easy to get wrong in a helper. This is less visible than Option 1's `.In(tx)`.
- **Breaks §8.3** ("database handles never travel on context"). The RFC would need an explicit carve-out: an opaque transaction marker, unreadable outside `pkg/storage` and `domains`.
- **Cost:** a `q(ctx)` helper per receiver, and every query call goes through it.

## Option 3: Transaction-aware connection wrapper (domains unchanged)

Push Option 2's resolution down into the connection itself. `storage.NewPostgres` returns a connection whose `Exec`/`Query`/`QueryRow` check `ctx` for an open transaction and route to it. Domain code is unchanged: `queries.New(db)` keeps working, and every query joins the active transaction.

```go
// pkg/storage: Postgres.Exec / Query / QueryRow
func (p *Postgres) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := txFrom(ctx); ok { // unexported; only pkg/storage reads it
		return tx.Query(ctx, sql, args...)
	}
	return p.pool.Query(ctx, sql, args...)
}

func (t Transactor) Run(ctx context.Context, fn func(ctx context.Context) error) error
```

- **R1:** strongest of all four. There is no exported way to read the transaction; it lives only inside `pkg/storage`.
- **R3/R5/R6:** the same as Option 2, and receivers have no `q(ctx)` or `In(tx)` helpers.
- **Footgun:** the same outer-`ctx` escape as Option 2.
- **Breaks §8.3** like Option 2, and it is the most "magic". Reading a domain method does not tell you whether it runs in a transaction, which works against §3.7 ("No hidden control flow").
- **Cost:** the smallest diff of the four, all in `pkg/storage`, plus a `Transactor`.

## Option 4: Unit-of-work binder on top of Option 1

Option 1, plus a generic helper so a service declares the transaction-scoped set it needs once. This removes `.In(tx)` repetition and the bypass footgun inside the callback.

```go
// pkg/storage
func Atomic[R any](
	ctx context.Context,
	t Transactor,
	bind func(tx Tx) R,
	fn func(ctx context.Context, r R) error,
) error
```

```go
// services/registration.go
type registerTx struct {
	users  domains.Users
	tokens domains.Tokens
	tx     storage.Tx // for queue.InsertTx
}

func (i Identity) bindRegister(tx storage.Tx) registerTx {
	return registerTx{users: i.users.In(tx), tokens: i.tokens.In(tx), tx: tx}
}

err := storage.Atomic(ctx, i.tx, i.bindRegister, func(ctx context.Context, r registerTx) error {
	created, err := r.users.Create(ctx /* … */) // cannot reach a non-tx receiver
	// …
	_, err = i.queue.InsertTx(ctx, r.tx, args, nil)
	return err
})
```

- **R1–R6:** the same as Option 1.
- **Footgun:** mostly removed, because the callback only sees tx-bound receivers. `i.users` is still reachable through the closure, but the natural path is `r.users`.
- **Cost:** a small bind struct per multi-domain use case. This fits the RFC's "use case lives in a service" framing, but it adds ceremony.

---

## Comparison

| | 1 Opaque handle | 2 Ctx in receivers | 3 Ctx-aware connection | 4 Binder on 1 |
| --- | --- | --- | --- | --- |
| Raw SQL impossible outside domains | lint on `Unwrap` | lint on `TxFrom` | by construction | lint on `Unwrap` |
| Transactional enqueue, no jobs in domains | `InsertTx(ctx, tx)` | automatic | automatic | `InsertTx(ctx, r.tx)` |
| Visible in the code | explicit `tx` | `ctx` only | invisible | explicit, bound once |
| Escape footgun | missing `.In(tx)` (lintable) | outer `ctx` | outer `ctx` | low |
| Nesting / composition | pass `tx` | joins automatically | joins automatically | pass `tx` |
| Adding a domain | `NewX(db)` in runtime | same | same | same |
| §8.3 (no DB on ctx) | ✅ | ❌ needs carve-out | ❌ needs carve-out | ✅ |
| Domain boilerplate | `In(tx)` per receiver | `q(ctx)` per receiver | none | `In(tx)` per receiver |
| `pkg/` changes | Transactor, Tx, Unwrap, InsertTx | Transactor, TxFrom, Insert | Transactor, Postgres methods, Insert | Option 1 + `Atomic` |

## Recommendation

**Option 1, with Option 4's `storage.Atomic` as an opt-in helper.** It is the only family that keeps §8.3 and §3.7 intact. The transaction is a visible value in every signature that uses it, and the one footgun is mechanically lintable. Jobs stay a service concern through `InsertTx`, and receivers go back to plain `NewUsers(db)` constructors listed individually.

Pick **Option 3** instead if you would rather amend §8.3 and accept invisible transactions in exchange for zero domain boilerplate and automatic joining. It is the smallest change and the hardest to misuse at the SQL boundary, but the hardest to read.

## What happens to the current changes

- **Reverted / replaced:** `domains_store.tmpl`, `domains_jobs.tmpl`, `Store` wiring in runtime, seeds, services, and the API handler, and the `Store` parts of the patterns, skills, README, RFC and fixtures.
- **Kept:** the validation consolidation (`Validate()` gate, `normaliseEmail`, `ChangePassword`, `// andurel:table`), the `verify` boundary, structured violations, the generated-drift step, exit code 9, and the manifest and `inspect domains` fixes.
- **Adapted:** the `tx-bypass` lint retargets to the chosen option: a missing `.In(tx)` for 1/4, an outer `ctx` captured in the callback for 2/3. The manifest's `store.go` edit entry becomes a runtime `NewX(db)` entry. `inspect domains` swaps `wired` (Store field) for "constructed in runtime".

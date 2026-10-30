# Testing

The scaffold's tests are the examples to copy. Real PostgreSQL integration
tests are the core of verification: a cheap static rule can guard a boundary,
but only a test against the real graph proves that a write commits, a rollback
leaves no rows, or an expiry boundary does what the code says.

## Race detector

`go test -race` needs cgo and a C compiler. Install a C toolchain, or use Zig as
the C compiler when the host has none:

```bash
CGO_ENABLED=1 go test -race -count=1 ./...

# No C compiler configured; zig is on PATH:
CC="$(command -v zig) cc" go test -race -count=1 ./...

# A compiler shim without zig registered: use the absolute path.
CC="/opt/zig/zig cc" go test -race -count=1 ./...
```

Without `CGO_ENABLED=1` and a working `CC`, `-race` is unsupported and the
toolchain reports it instead of silently running without the detector. Run the
race suite for code that shares mutable state (caches, registries, pools), not
only for the final acceptance run.

## Deterministic clocks

Do not call `time.Now()` in code whose boundary a test asserts. Two patterns
match the scaffold:

- A method that compares one instant takes `now time.Time` explicitly.
  `domains.Tokens.DestroyExpired(ctx, before)` is the scaffold example, and a
  test passes an instant instead of reading the wall clock.
- A service or handler that needs several timestamps takes a clock function,
  defaulted once at construction:

```go
type Clock func() time.Time

func NewTokens(now Clock) Tokens {
	if now == nil {
		now = time.Now
	}
	return Tokens{now: now}
}
```

Tests pass a fixed closure (`func() time.Time { return fixed }`); production
passes `time.Now` from `app`. Prefer the explicit `now` parameter when one call
is enough; it makes the dependency visible at the call site.

The validation package follows the same rule:

```go
now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
builder := validation.NewBuilder()
builder.TimeBetween("expiresAt", expiresAt, now, 0, 24*time.Hour)
if err := builder.Err(); err != nil {
	// expiresAt is outside [now, now+24h]
}
```

Assert both sides of the boundary with the same `now`; do not build fixtures
from `time.Now().Add(...)` and then compare against another `time.Now()`.

## Capability tokens

`pkg/auth` generates prefixed, URL-safe capability tokens and returns the three
values a service stores: the plaintext to hand to the user, the short display
prefix, and the SHA-256 hash for lookup:

```go
plain, displayPrefix, hash, err := auth.NewCapabilityToken("report_", 32)

// Persist hash and displayPrefix; return plain once and never log it.
if auth.VerifyCapabilityToken(presented, storedHash) {
	// constant-time match
}
```

Entropy must be at least 16 bytes (128 bits). `CapabilityTokenDisplayLength`
is the display prefix length. Test the service with a token captured from the
creation call, assert the stored hash is not the plaintext, and assert a
mutated token fails `VerifyCapabilityToken`. Store the hash in a `bytea`
column with a unique index so lookup is by hash, not by plaintext.

## Test databases

Each test package owns one `storagetest.Cluster` in `TestMain`:

```go
var testCluster *storagetest.Cluster

func TestMain(m *testing.M) {
	storagetest.Main(m, &testCluster)
}
```

Without Docker the helper reports why and exits 0, so a developer run still
passes and `andurel verify --with-tests` reports the package's skipped tests.
`NewTestDB` creates a fresh isolated database and migrates it to head; tests
can run in parallel.

To test a migration, start at a version and continue later:

```go
db := testCluster.NewTestDBAtVersion(t, migrations.Migrations, ".", 2)
// Insert rows that schema version 2 accepts, or assert the old shape.
storagetest.MigrateTestDB(t, db, migrations.Migrations, ".", storagetest.MigrationHead)
// Assert the upgraded schema and preserved data.
```

`storagetest.MigrationHead` (zero) means every migration; `MigrateTestDB` takes
the same arguments as `NewTestDB` plus the target version.

testcontainers shares one Ryuk session per parent `go test` process. Under
parallel packages, a package that finishes can reap a sibling's container. Use
`go test -p 1 ./...` or `TESTCONTAINERS_RYUK_DISABLED=true` when DB-test
packages run concurrently; close the cluster through `storagetest.Main`.

## Fault injection and rollback

`storagetest.FailingTransactor` wraps a real connection and fails one selected
statement inside the transaction. Pass it to the service under test; the
service opens the transaction normally, and `AtomicOps` must roll back every
write before the injected error:

```go
transactor := storagetest.FailingTransactor(db, storagetest.StatementFault{
	FailAt: 1,
	Match:  func(query string) bool { return strings.Contains(query, "INSERT INTO users") },
})
identity := services.NewIdentity(transactor, domains.NewUsers(db), domains.NewTokens(db), queueInsert, authCfg, mailCfg, hosts)

_, err := identity.RegisterUser(ctx, services.RegisterUserData{
	Email:           "rollback@example.com",
	Password:        "correct horse battery staple",
	ConfirmPassword: "correct horse battery staple",
})
if !errors.Is(err, storagetest.ErrFaultInjected) {
	t.Fatalf("RegisterUser error = %v, want the injected fault", err)
}
if _, err := domains.NewUsers(db).FindByEmail(ctx, "rollback@example.com"); !errors.Is(err, domains.ErrNotFound) {
	t.Fatalf("row survived the rollback: %v", err)
}
```

`StatementFault.FailAt` is the 1-based number of a matching statement; zero or
negative fails the first match. `Match` filters by SQL text and nil matches
every statement. `StatementFault.Err` replaces `ErrFaultInjected` when the test
needs a typed error. Pass the test context to the call, then
verify with a second connection or receiver so the assertion cannot see the
failed transaction's uncommitted state.

## Read-only and isolated transactions

`storage.AtomicOps` accepts transaction options for reads that must not write
or must see one snapshot:

```go
err := storage.AtomicOps(ctx, report.transactor, func(ctx context.Context, tx storage.Tx) error {
	users := report.users.InTx(tx)
	tokens := report.tokens.InTx(tx)

	// Read users, then tokens. Both reads see the same snapshot.
	return nil
}, storage.ReadOnly(), storage.Isolation(storage.RepeatableRead))
```

`storage.ReadOnly()` makes any write fail instead of committing.
`storage.Isolation(storage.RepeatableRead)` keeps a multi-statement read on one
PostgreSQL snapshot; `Serializable` is available when the read must be
serializable with concurrent writers. Options apply only to an unbound
`Transactor`: a receiver already inside a caller's transaction rejects them.
Tests should assert that a write attempted through a read-only transaction
returns an error and leaves no row, and that a concurrent committed writer does
not change the second read in a repeatable-read transaction.

## Running the suite

```bash
go test ./...
go test -race -count=1 ./...
go test -p 1 ./...            # DB packages with one test container session
andurel verify --with-tests   # reports skipped_packages and skipped_tests
```

Tests that need built frontend assets should `t.Skip` with the install hint
when `assets/dist` is missing, then be re-run after the frontend build for the
final evidence. A skip is reported, not silently treated as a pass.

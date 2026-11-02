# Atomic multi-table seed

Use exported domain receivers and a shared transaction when a named seed writes
multiple tables: either every write commits or none does.

## Files

### seeds/demo.go (fragment)
```go
func Demo(ctx context.Context, db storage.Connection) error {
	transactor := storage.NewTransactor(db)
	users := domains.NewUsers(db)
	products := domains.NewProducts(db)
	return storage.AtomicOps(ctx, transactor, func(ctx context.Context, tx storage.Tx) error {
		users, products := users.InTx(tx), products.InTx(tx)
		// Look up existing records and create missing rows using these bound receivers.
		_ = users
		_ = products
		return nil
	})
}
```

### seeds/registry.go (fragment)
```go
// Register Demo in seeds.Registry under a stable lowercase name.
```

## Rules

- The Products receiver is illustrative; create the domain and migration first or substitute the app's own domains.
- Bind every participating receiver with `InTx(tx)` inside `storage.AtomicOps` and return any error to roll back all writes. Do not use unbound receivers in the callback.
- Make the seed idempotent by looking up existing rows before creating them, as the scaffold's `createUser` does. Use domain APIs rather than direct SQL.
- Register the seed in `seeds.Registry`; check `andurel db seed --list` and run it with `andurel db seed <name>`.
- Single-table scaffold seeds do not need a multi-domain transaction.

## Checklist

- [ ] The named seed appears in `andurel db seed --list` and can be run twice safely.
- [ ] Every multi-table write uses transaction-bound receivers and propagates errors.
- [ ] A failure after an earlier write leaves no partial data.

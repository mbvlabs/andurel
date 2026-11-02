# Migration

A migration is a goose SQL file under `migrations/`, created by
`andurel generate migration <name>`. The migration is the schema source of
truth; domain entities and queries follow it.

## Files

### migrations/<timestamp>_create_<table>.sql
```sql
-- +goose Up
create table products (
	id uuid primary key default gen_random_uuid(),
	name text not null unique,
	price_cents bigint not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index products_created_at_idx on products (created_at desc);

-- +goose Down
drop table if exists products;
```

## Rules

- Create migrations with `andurel generate migration <name>`; never invent file names.
- Always write both `-- +goose Up` and `-- +goose Down`.
- Apply with `andurel db migrate up`; check status with `andurel db migrate status`.
- Keep one logical schema change per migration.
- After a schema change, update the owning domain entity and colocated `.sql`.
- Existing databases need reconciliation before `andurel db migrate up`: goose skips recorded versions, not unrecorded tables. Check migration history, existing relations and dependencies first; neither a recorded version nor manually marking a version applied proves schema equivalence. Do not blindly baseline or apply migrations. Never rewrite or remove migrations already applied to any deployed database.
- The scaffold's `domains/user.sql`, `domains/token.sql`, entities and authentication code require its auth schema and semantics. If retaining scaffold auth, reconcile the existing schema and data with those contracts, then use additive migrations for additional tables and columns (nullable or with a suitable default). Preserve required columns, types, constraints and auth behavior. Keep River migrations `00001`–`00007` and any other required bootstrap system migrations; inspect for conflicts with existing system objects too.
- If adapting incompatible auth instead, replace the *unapplied* scaffold auth migrations `migrations/00008_create_users_table.sql` and `migrations/00009_create_tokens_table.sql` only together with the affected domain entities, SQL, generated queries and authentication consumers. Supply a complete schema history that narsilc can analyze, and plan data conversion and rollback. Do not drop River or required bootstrap migrations. Existing deployments with recorded versions require a separately designed forward-only migration strategy.
- Test the full reconciliation on a disposable clone first, including migration history, data, dependencies, constraints, permissions and application behavior. Run `andurel sync queries --json` and `andurel verify` as well as database and auth tests; these checks alone do not prove live schema or data compatibility. Review backup, cutover and rollback with the database owner before touching a real database.

## Example: isolate an unrelated existing users table

This narrowly scoped example starts with a **disposable** PostgreSQL database with no goose history, an empty `users (id uuid PRIMARY KEY)` table, no `tokens`, River or other scaffold objects, and no dependencies or consumers of `users`. It preserves that table under another name; it does **not** migrate users, passwords, sessions or existing authentication. It is not a general solution for a populated or dependent table.

1. On the clone, inspect `andurel db migrate status` and PostgreSQL's catalog, including dependent foreign keys, views, triggers, indexes and permissions. Confirm the assumptions above and that `legacy_users` is free. Stop if they do not hold.
2. Using `psql` connected **only to the clone**, run `ALTER TABLE users RENAME TO legacy_users;`. This is an explicit pre-migration cutover step, not a goose baseline. Keep the scaffold migrations `00001`–`00009` unchanged. A rename within migration 8 is not a substitute here: narsilc analyzes migrations from an empty schema and cannot resolve that external table.
3. Run `andurel db migrate up` and `andurel db migrate status` against the clone. Inspect the resulting `users`, `tokens`, `legacy_users`, River objects, constraints and migration history; run `andurel sync queries --json`, `andurel verify`, and application/auth tests. The new `users` and `tokens` belong to the scaffold; the old table remains `legacy_users`.
4. For a real port, design and test a separate data/auth migration and a coordinated cutover for all consumers. A rename changes what existing consumers see; PostgreSQL dependencies can continue to point to the renamed table. Do not run this example against a real database without an application-specific plan and tested backup and rollback. Goose Down for scaffold migration 8 drops the **new** `users` table and does not undo the separate rename; do not use reset/down as a rollback plan.

## Checklist

- [ ] File created by `andurel generate migration`.
- [ ] `Up` and `Down` sections present.
- [ ] Applied locally with `andurel db migrate up`.
- [ ] Owning domain entity and `.sql` updated to match.

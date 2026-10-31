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
- Porting onto an existing database: the scaffold's `users` and `tokens` queries (`domains/user.sql`, `domains/token.sql`) and their entities assume the scaffold's own tables. Replacing those tables breaks `andurel sync queries` and `andurel verify` project-wide. Either keep the two scaffold migrations (`migrations/00008_create_users_table.sql` and `migrations/00009_create_tokens_table.sql`, after the seven River migrations `00001`–`00007`) and add your schema beside them, or drop both and rewrite the `users`/`tokens` domain files, their `.sql`, and every query that reads them in the same change.
- Keep scaffold auth on an existing database with additive migrations: add columns with `add column` (nullable, or with a default) and new tables with `create table`. Do not rename or retype a column the auth domains read, and do not drop the scaffold's River migrations, which the queue depends on.
- Apply a port's migrations to a database that already has the tables with `andurel db migrate up`; goose records each version, so a migration is never re-run. Check which versions are already recorded with `andurel db migrate status` before deleting any migration file.

## Checklist

- [ ] File created by `andurel generate migration`.
- [ ] `Up` and `Down` sections present.
- [ ] Applied locally with `andurel db migrate up`.
- [ ] Owning domain entity and `.sql` updated to match.

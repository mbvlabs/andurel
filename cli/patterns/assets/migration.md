# Migration

A migration is a goose SQL file under `migrations/`, created by
`andurel generate migration <name>`. The migration is the schema source of
truth; domain entities and queries follow it.

## Files

### migrations/<timestamp>_create_<table>.sql
```sql
-- +goose Up
create table posts (
	id uuid primary key default gen_random_uuid(),
	title text not null,
	body text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index posts_created_at_idx on posts (created_at desc);

-- +goose Down
drop table if exists posts;
```

## Rules

- Create migrations with `andurel generate migration <name>`; never invent file names.
- Always write both `-- +goose Up` and `-- +goose Down`.
- Apply with `andurel db migrate up`; check status with `andurel db migrate status`.
- Keep one logical schema change per migration.
- After a schema change, update the owning domain entity and colocated `.sql`.

## Checklist

- [ ] File created by `andurel generate migration`.
- [ ] `Up` and `Down` sections present.
- [ ] Applied locally with `andurel db migrate up`.
- [ ] Owning domain entity and `.sql` updated to match.

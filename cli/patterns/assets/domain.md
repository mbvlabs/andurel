# Domain

A domain is the complete database and business picture for one entity: its
table, its colocated SQL, its entity type with field tags, its receiver
methods, and the domain errors it returns. Only `domains/**` imports the
generated `domains/internal/queries` package; services, handlers, and runtime
call the domain constructor instead.

Default to type-driven work: the Go entity (its fields and tags) is the source
of truth. Change the entity first, then let the SQL and generated queries
follow. Hand-written SQL remains possible, but it is the exception.

## Files

### domains/errors.go
```go
package domains

import "errors"

var (
	ErrNotFound  = errors.New("domain: not found")
	ErrConflict  = errors.New("domain: conflict")
	ErrViolation = errors.New("domain: violation")
)

// ValidationError carries field-level messages that handlers map to 422 /
// Inertia props.errors.
type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string { return "domain: validation failed" }

func (e ValidationError) Is(target error) bool { return target == ErrViolation }
```

### domains/<entity>/<entity>.go
```go
package user

// andurel:table users

import (
	"context"
	"errors"
	"uuid"

	"app/domains"
	"app/domains/internal/queries"

	"github.com/jackc/pgx/v5"
	"github.com/mbvlabs/andurel/pkg/storage"
)

type Users struct{ q *queries.Queries }

func NewUsers(db storage.Connection) Users          { return Users{q: queries.New(db)} }
func (u Users) WithTx(tx storage.Transaction) Users { return Users{q: queries.New(tx)} }

type User struct {
	ID    uuid.UUID `andurel:"id"`
	Email string    `andurel:"email"`
}

func (u Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
	entity, err := u.q.GetUser[User](ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, domains.ErrNotFound
	}
	return entity, err
}

func (u Users) Create(ctx context.Context, data CreateUserData) (User, error) {
	// validate, then persist through the generated query and return the entity.
	return u.q.CreateUser[User](ctx, queries.CreateUserParams{
		Email: data.Email,
	})
}
```

### domains/<entity>/<entity>.sql
```sql
-- name: GetUser :one
select * from users where id = $1;

-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: CreateUser :one
insert into users (email) values ($1) returning *;

-- name: ListUsers :many
select * from users order by created_at desc limit $1 offset $2;
```

### domains/<entity>/<entity>_test.go
```go
package user_test

import (
	"testing"
	"uuid"

	"app/domains/user"
	"app/migrations"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestUsers_Create(t *testing.T) {
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	users := user.NewUsers(db)

	got, err := users.Create(t.Context(), user.CreateUserData{Email: "a@b.c"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID == uuid.Nil() {
		t.Fatal("expected id")
	}
}
```

## Rules

- A domain package must not import another domain package. Cross-entity rules belong in a service.
- Only files under `domains/` may import `domains/internal/queries`.
- SQL lives beside the domain that owns it (`domains/user/user.sql`); narsilc globs `domains/**/*.sql`.
- Return `domains.ErrNotFound`, `domains.ErrConflict`, and `domains.ErrViolation`, never raw pgx errors.
- Use `storage.RunInTransaction` (or `WithTx`) for multi-statement writes.
- Keep the entity 1:1 with a scanned row; model transformations in a small `Transform` method.

## Checklist

- [ ] Entity type exists under `domains/<entity>/` with `andurel` field tags.
- [ ] `<entity>.sql` contains every query the receiver calls.
- [ ] Receiver exposes `New<Entity>(db)` and a `WithTx` variant.
- [ ] Not-found maps to `domains.ErrNotFound`; conflicts to `domains.ErrConflict`.
- [ ] No import of another domain package.
- [ ] `andurel sync queries` succeeds and `andurel verify` passes.

# Domain

A domain is the complete database and business picture for one entity: its
table, its colocated SQL, its newtype entity with `Parse` constructors, its
receiver methods, and the domain errors it returns. Domains live in one flat
`domains` package — one file per domain, no sub-packages. Only `domains`
imports the generated `domains/internal/queries` package; services, handlers,
and runtime call domain constructors instead.

Default to type-driven work: the Go entity (its newtypes and tags) is the
source of truth. Change the entity first, then let the SQL and generated
queries follow. Hand-written SQL remains possible, but it is the exception.
Parse, don't validate: an invalid value is unrepresentable, so callees never
re-check.

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

### domains/<entity>.go
```go
package domains

// andurel:table users

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"uuid"

	"app/domains/internal/queries"

	"github.com/jackc/pgx/v5"
	"github.com/mbvlabs/andurel/pkg/storage"
)

type Users struct{ q *queries.Queries }

func NewUsers(db storage.Connection) Users          { return Users{q: queries.New(db)} }
func (u Users) WithTx(tx storage.Transaction) Users { return Users{q: queries.New(tx)} }

// Email is a parsed, normalised address. Build one with ParseEmail.
type Email string

func ParseEmail(raw string) (Email, error) {
	normalised := strings.ToLower(strings.TrimSpace(raw))
	if normalised == "" {
		return "", ValidationError{Fields: map[string]string{"email": "Email is required"}}
	}
	if _, err := mail.ParseAddress(normalised); err != nil {
		return "", ValidationError{Fields: map[string]string{"email": "Email must be a valid email address"}}
	}
	return Email(normalised), nil
}

func (e Email) String() string { return string(e) }

type PasswordHash []byte

// HashPassword salts and hashes a password with the pepper.
func HashPassword(password, pepper string) (PasswordHash, error) {
	return PasswordHash("hashed:" + password + pepper), nil
}

func (h PasswordHash) Verify(providedPassword, pepper string) (bool, error) {
	candidate, err := HashPassword(providedPassword, pepper)
	if err != nil {
		return false, err
	}
	return string(candidate) == string(h), nil
}

type User struct {
	ID       uuid.UUID    `andurel:"id"`
	Email    Email        `andurel:"email"`
	Password PasswordHash `andurel:"password"`
}

func (u *User) Transform(row queries.UserRow) error {
	u.ID = row.ID
	u.Email = Email(row.Email)
	u.Password = PasswordHash(row.Password)
	return nil
}

type CreateUserData struct {
	Email Email
}

func (u Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
	entity, err := u.q.GetUser[User](ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return entity, err
}

func (u Users) Create(ctx context.Context, data CreateUserData) (User, error) {
	return u.q.CreateUser[User](ctx, queries.CreateUserParams{
		Email: string(data.Email),
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

### domains/<entity>_test.go
```go
package domains_test

import (
	"testing"
	"uuid"

	"app/domains"
	"app/migrations"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestParseEmail(t *testing.T) {
	if _, err := domains.ParseEmail("not-an-email"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUsers_Create(t *testing.T) {
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	users := domains.NewUsers(db)

	email, err := domains.ParseEmail("a@b.c")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := users.Create(t.Context(), domains.CreateUserData{Email: email})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID == uuid.Nil() {
		t.Fatal("expected id")
	}
}
```

## Rules

- One file per domain in the flat `domains` package. Cross-entity rules belong in a service.
- Only files under `domains/` may import `domains/internal/queries`.
- One newtype per invariant; `ParseX` is the supported constructor; read access is read-only. There is no post-construction `Validate` method.
- Entities that carry newtypes implement `Transform` for their generated row type; rows stay primitive.
- Convert to primitives (`string(...)`) at the narsilc call boundary.
- SQL lives beside the domain that owns it (`domains/user.sql`); narsilc globs `domains/**/*.sql`.
- Return `ErrNotFound`, `ErrConflict`, and `ErrViolation`, never raw pgx errors.
- Use `storage.RunInTransaction` (or `WithTx`) for multi-statement writes.
- Keep the entity 1:1 with a scanned row; model transformations in a small `Transform` method.

## Checklist

- [ ] Entity inputs are parsed types built by `ParseX`, never raw strings at the domain boundary.
- [ ] No post-construction `Validate` method; invalid values are unrepresentable.
- [ ] Entity implements `Transform` for its generated row when it carries newtypes.
- [ ] `<entity>.sql` contains every query the receiver calls.
- [ ] Receiver exposes `New<Entity>(db)` and a `WithTx` variant.
- [ ] Not-found maps to `ErrNotFound`; conflicts to `ErrConflict`.
- [ ] `andurel patterns show domain --entity <Name>` manifest applied in order.
- [ ] `andurel sync queries` succeeds and `andurel verify` passes.

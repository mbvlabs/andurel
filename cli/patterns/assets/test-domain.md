# Test domain

A domain test runs against a real Postgres database with migrations applied.
There are no factories: build entities inline through the domain API. One
clean database per test keeps cases independent.

## Files

### domains/<entity>/<entity>_test.go
```go
package user_test

import (
	"testing"

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
	if got.Email != "a@b.c" {
		t.Fatalf("email = %q, want %q", got.Email, "a@b.c")
	}
}
```

## Rules

- Domain tests require a real Postgres container; handler tests do not.
- Get a clean database per test with `storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")`.
- Build entities inline through the domain API; do not add factories.
- Assert on the entity returned by the domain, not raw SQL rows.
- Keep tests in the `_test` package to exercise the public API.

## Checklist

- [ ] One clean database per test via `NewTestDB`.
- [ ] No factory package used.
- [ ] Success and error paths both covered.
- [ ] `go test ./domains/...` passes.

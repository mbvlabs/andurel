# Test domain

A domain test runs against a real Postgres database with migrations applied.
There are no factories: build entities inline through the domain API. One
clean database per test keeps cases independent. Parse constructors get plain
unit tests that need no database at all.

## Files

### domains/<entity>/<entity>_test.go
```go
package domains_test

import (
	"testing"

	"app/domains"
	"app/migrations"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestParseEmail(t *testing.T) {
	for _, raw := range []string{"", "  ", "not-an-email", "a@b"} {
		if _, err := domains.ParseEmail(raw); err == nil {
			t.Fatalf("ParseEmail(%q) succeeded, want error", raw)
		}
	}

	email, err := domains.ParseEmail("  A@B.c ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if email.String() != "a@b.c" {
		t.Fatalf("email = %q, want normalised %q", email, "a@b.c")
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
	if got.Email != email {
		t.Fatalf("email = %q, want %q", got.Email, email)
	}
}
```

## Rules

- Domain tests require a real Postgres container; handler tests do not.
- Unit-test every `ParseX` constructor directly, including normalisation and the unhappy paths.
- Get a clean database per test with `storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")`.
- Build entities inline through the domain API; do not add factories.
- Assert on the entity returned by the domain, not raw SQL rows.
- Keep tests in the `_test` package to exercise the public API.

## Checklist

- [ ] `ParseX` unit tests cover valid, normalising, and invalid inputs.
- [ ] One clean database per test via `NewTestDB`.
- [ ] No factory package used.
- [ ] Success and error paths both covered.
- [ ] `go test ./domains/...` passes.

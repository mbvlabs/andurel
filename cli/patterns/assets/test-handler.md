# Test handler

A handler test builds a real `httptest` server from actual registrations using
`pkg/anduin/anduintest`. It needs nothing external, so it runs without
Postgres.

## Files

### handlers/user_test.go
```go
package handlers_test

import (
	"net/http"
	"testing"

	"app/domains"
	"app/handlers"
	"app/migrations"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/anduin/anduintest"
	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestUserIndex(t *testing.T) {
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	users := domains.NewUsers(db)
	for i := range 3 {
		email, err := domains.ParseEmail("u@x.y")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := users.Create(t.Context(), domains.CreateUserData{Email: email}); err != nil {
			t.Fatal(err)
		}
	}

	srv := anduintest.New(t, func(rt *anduin.Router) {
		handlers.NewUserHandler(users, testRenderer(t)).Register(rt)
	})

	resp := srv.Get(routes.UserIndex.Path() + "?page=1")
	resp.AssertStatus(http.StatusOK)
	resp.AssertInertia("User/Index")
}
```

## Rules

- Use `anduintest.New(t, register)` to build the server from real registrations.
- Provide a local `testRenderer(t)` helper that builds an `*inertia.Renderer`; there is no framework default.
- Call `srv.Get(...)`, `srv.PostForm(...)`, or `srv.PostJSON(...)`; responses assert with `AssertStatus(int)` and `AssertInertia(string)`.
- Prefer handler tests over integration tests when no cross-domain behavior is exercised.
- Assert status and the Inertia component name, not HTML strings.
- Seed data through the domain API.

## Checklist

- [ ] Server built with `anduintest.New`.
- [ ] Response asserted by status and Inertia component.
- [ ] Data seeded through domain constructors.
- [ ] `go test ./handlers/...` passes.

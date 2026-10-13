# Test handler

A handler test builds a real `httptest` server from actual registrations using
`pkg/anduin/anduintest`. It needs nothing external, so it runs without
Postgres.

## Files

### handlers/<resource>/handler_test.go
```go
package user_test

import (
	"net/http"
	"testing"

	"app/domains/user"
	userhandler "app/handlers/user"
	"app/migrations"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/anduin/anduintest"
	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestUserIndex(t *testing.T) {
	db := storage.NewTestCluster(t.Context()).NewTestDB(t, migrations.Migrations, ".")
	users := user.NewUsers(db)
	for i := range 3 {
		if _, err := users.Create(t.Context(), user.CreateUserData{Email: "u@x.y"}); err != nil {
			t.Fatal(err)
		}
	}

	srv := anduintest.New(t, func(rt *anduin.Router) {
		userhandler.NewHandler(users, testRenderer(t)).Register(rt)
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

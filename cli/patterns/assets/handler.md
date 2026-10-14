# Handler

A handler is a thin HTTP adapter in the single flat `handlers` package. Its
methods are regular Go over `context.Context`, and `pkg/anduin` supplies
request and response access from that context. Each handler declares its
dependencies in a distinct `NewXHandler` constructor and registers its routes
in `Register`. There are no handler sub-packages: `NewUserHandler`,
`NewAuthHandler`, and `NewAPIHandler` disambiguate instead of imports.

Handlers parse raw input at the edge: payload strings become domain newtypes
via `ParseX` before any service call, so services and domains never re-check.

## Files

### handlers/user.go
```go
package handlers

import (
	"context"

	"app/domains"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

type UserHandler struct {
	users    domains.Users
	renderer *inertia.Renderer
}

func NewUserHandler(users domains.Users, renderer *inertia.Renderer) UserHandler {
	return UserHandler{users: users, renderer: renderer}
}

func (h UserHandler) Register(rt *anduin.Router) {
	rt.Get(routes.UserIndex.Path(), h.Index)
	rt.Get(routes.UserShow.Path(), h.Show)
	rt.Post(routes.UserCreate.Path(), h.Create)
}

func (h UserHandler) Index(ctx context.Context) error {
	page, err := anduin.QueryInt(ctx, "page", 1)
	if err != nil {
		return anduin.BadRequest(err)
	}

	result, err := h.users.Paginate(ctx, page, 25)
	if err != nil {
		return err
	}

	return h.renderer.Page(ctx, "User/Index", inertia.FromStruct(result.Items)).Render()
}

func (h UserHandler) Create(ctx context.Context) error {
	var payload struct {
		Email string `json:"email"`
	}
	if err := anduin.DecodeJSON(ctx, &payload); err != nil {
		return anduin.BadRequest(err)
	}

	email, err := domains.ParseEmail(payload.Email)
	if err != nil {
		return err
	}

	created, err := h.users.Create(ctx, domains.CreateUserData{Email: email})
	if err != nil {
		return err
	}

	return h.renderer.Redirect(ctx, routes.UserShow.URL(created.ID.String()))
}
```

### handlers/errors.go
```go
package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

func MapError(renderer *inertia.Renderer) anduin.ErrorHandler {
	return func(ctx context.Context, err error) error {
		switch {
		case errors.Is(err, domains.ErrNotFound):
			return renderer.Page(ctx, "Errors/NotFound", nil).
				Status(http.StatusNotFound).Render()
		case errors.Is(err, domains.ErrViolation):
			if ve, ok := errors.AsType[domains.ValidationError](err); ok {
				return renderer.Page(ctx, "Errors/Validation", nil).
					ValidationErrors(ve.Fields).
					Status(http.StatusUnprocessableEntity).
					Render()
			}
		}
		return anduin.Error(ctx, http.StatusInternalServerError, err)
	}
}
```

## Rules

- A handler signature is `func(ctx context.Context) error`.
- One flat `package handlers`; distinct `NewXHandler` constructors, no sub-packages.
- Parse payloads into domain newtypes at the edge with `ParseX`; pass parsed values down.
- Never import `domains/internal/queries`; call domain methods.
- Route registration happens in `Register(rt *anduin.Router)`.
- Return domain errors unchanged; the router error handler maps them uniformly.
- Construct the handler once in `internal/runtime` and call `Register`.

## Checklist

- [ ] `NewXHandler` constructor lists every dependency.
- [ ] `Register` binds each route to a method.
- [ ] Raw payload input is parsed with `ParseX` before any service or domain call.
- [ ] Handler methods return domain errors untouched.
- [ ] Wired via `handlers.NewUserHandler(...).Register(rt)` in `internal/runtime`.
- [ ] `andurel inspect handlers --json` lists the new handler.

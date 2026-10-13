# Handler

A handler is a thin HTTP adapter. Its methods are regular Go over
`context.Context`, and `pkg/anduin` supplies request and response access from
that context. Each handler declares its dependencies in a `NewHandler`
constructor and registers its routes in `Register`.

## Files

### handlers/<resource>/handler.go
```go
package user

import (
	"context"

	domainuser "app/domains/user"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

type Handler struct {
	users    domainuser.Users
	renderer *inertia.Renderer
}

func NewHandler(users domainuser.Users, renderer *inertia.Renderer) Handler {
	return Handler{users: users, renderer: renderer}
}

func (h Handler) Register(rt *anduin.Router) {
	rt.Get(routes.UserIndex.Path(), h.Index)
	rt.Get(routes.UserShow.Path(), h.Show)
	rt.Post(routes.UserCreate.Path(), h.Create)
}

func (h Handler) Index(ctx context.Context) error {
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
- Never import `domains/internal/queries`; call domain methods.
- Route registration happens in `Register(rt *anduin.Router)`.
- Return domain errors unchanged; the router error handler maps them uniformly.
- Construct the handler once in `internal/runtime` and call `Register`.

## Checklist

- [ ] `NewHandler` constructor lists every dependency.
- [ ] `Register` binds each route to a method.
- [ ] Handler methods return domain errors untouched.
- [ ] Wired via `userhandlers.NewHandler(...).Register(rt)` in `internal/runtime`.
- [ ] `andurel inspect handlers --json` lists the new handler.

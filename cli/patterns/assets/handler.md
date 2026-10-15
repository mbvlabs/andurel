# Handler

A handler is a thin HTTP adapter in the single flat `handlers` package. Its
methods are regular Go over `context.Context`, and `pkg/anduin` supplies
request and response access from that context. Each handler declares its
dependencies in a distinct `NewXHandler` constructor and registers its routes
in `Register`. There are no handler sub-packages: `NewProductHandler`,
`NewAuthHandler`, and `NewAPIHandler` disambiguate instead of imports.

Handlers decode the request and pass plain values down. They do not validate
entity rules: the domain's `Create`/`Update` is the single validation gate and
returns a `ValidationError` that `MapError` renders as a 422.

## Files

### handlers/product.go
```go
package handlers

import (
	"context"

	"app/domains"
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

type ProductHandler struct {
	products domains.Products
	renderer *inertia.Renderer
}

func NewProductHandler(products domains.Products, renderer *inertia.Renderer) ProductHandler {
	return ProductHandler{products: products, renderer: renderer}
}

func (h ProductHandler) Register(rt *anduin.Router) {
	rt.Get(routes.ProductIndex.Path(), h.Index)
	rt.Post(routes.ProductCreate.Path(), h.Create)
}

func (h ProductHandler) Index(ctx context.Context) error {
	page, err := anduin.QueryInt(ctx, "page", 1)
	if err != nil {
		return anduin.BadRequest(err)
	}

	result, err := h.products.Paginate(ctx, int64(page), 25)
	if err != nil {
		return err
	}

	return h.renderer.Page(ctx, "Product/Index", inertia.FromStruct(result)).Render()
}

func (h ProductHandler) Create(ctx context.Context) error {
	var payload struct {
		Name       string `json:"name"`
		PriceCents int64  `json:"priceCents"`
	}
	if err := anduin.DecodeJSON(ctx, &payload); err != nil {
		return anduin.BadRequest(err)
	}

	// No validation here: Products.Create validates and returns a
	// ValidationError that MapError renders as a 422.
	created, err := h.products.Create(ctx, domains.CreateProductData{
		Name:       payload.Name,
		PriceCents: payload.PriceCents,
	})
	if err != nil {
		return err
	}

	return h.renderer.Redirect(ctx, routes.ProductShow.URL(created.ID.String()))
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
- Decode payloads at the edge (`anduin.DecodeJSON`/`DecodeForm`); a decode failure is `anduin.BadRequest`. Do not re-implement entity validation.
- Depend on the narrowest thing: a receiver (`domains.Products`) for single-domain CRUD, a service for multi-domain use cases. Never `storage.*` or `domains/internal/queries`.
- Route registration happens in `Register(rt *anduin.Router)`.
- Return domain errors unchanged; the router error handler maps them uniformly.
- Construct the handler once in `internal/runtime` (`handlers.NewProductHandler(products, renderer).Register(router)`, with `products := domains.NewProducts(db)`).

## Checklist

- [ ] `NewXHandler` constructor lists every dependency.
- [ ] `Register` binds each route to a method.
- [ ] Payloads are decoded, not re-validated; validation errors come from the domain.
- [ ] Handler methods return domain errors untouched.
- [ ] Wired via `handlers.NewProductHandler(products, renderer).Register(router)` in `internal/runtime`.
- [ ] `andurel inspect handlers --json` lists the new handler.

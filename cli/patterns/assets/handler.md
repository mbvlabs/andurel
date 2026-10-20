# Handler

A handler is a thin HTTP adapter in the single flat `handlers` package. Its
methods are regular Go over `context.Context`, and `pkg/anduin` supplies
request and response access from that context. Each handler declares its
dependencies in a distinct `NewXHandler` constructor and registers its routes
in `Register`. There are no handler sub-packages: `NewProductHandler`,
`NewAuthHandler`, and `NewAPIHandler` disambiguate instead of imports.

Handlers decode the request and pass plain values down. They do not validate
entity rules: the domain's `Create`/`Update` is the single validation gate and
returns a `pkg/validation` error that `MapError` renders as a 422.

## Files

### handlers/product.go
```go
package handlers

import (
	"context"
	"net/http"

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

// CreateProductPayload is named so andurel sync payloads emits a matching
// TypeScript type for the page's form.
type CreateProductPayload struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
}

func (h ProductHandler) Create(ctx context.Context) error {
	var payload CreateProductPayload
	if err := anduin.DecodeJSON(ctx, &payload); err != nil {
		return anduin.BadRequest(err)
	}

	// No validation here: Products.Create owns entity validation, and MapError
	// renders pkg/validation errors as a 422.
	created, err := h.products.Create(ctx, domains.CreateProductData{
		Name:       payload.Name,
		PriceCents: payload.PriceCents,
	})
	if err != nil {
		return err
	}

	return h.renderer.Redirect(
		ctx,
		routes.ProductShow.URL(created.ID.String()),
		http.StatusSeeOther,
	)
}
```

### handlers/errors.go (excerpt)

An excerpt of the scaffold's own `handlers/errors.go`; `wantsJSON` and
`mapJSONError` are defined further down that file.

```go fragment
package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
	"github.com/mbvlabs/andurel/pkg/validation"
)

func MapError(renderer *inertia.Renderer) anduin.ErrorHandler {
	return func(ctx context.Context, err error) error {
		if wantsJSON(ctx) { // no X-Inertia header, Accept: application/json
			return mapJSONError(ctx, err) // same mapping, JSON bodies
		}
		if validationErr, ok := validation.As(err); ok {
			return renderer.Page(ctx, "Errors/Validation", nil).
				ValidationErrors(validationErr.ToMap()).
				Status(http.StatusUnprocessableEntity).
				Render()
		}

		switch {
		case errors.Is(err, domains.ErrNotFound):
			return renderer.Page(ctx, "Errors/NotFound", nil).
				Status(http.StatusNotFound).Render()
		case errors.Is(err, domains.ErrConflict):
			return renderer.Page(ctx, "Errors/Conflict", nil).
				Status(http.StatusConflict).Render()
		}
		return err // keeps its own status; the router answers and logs it
	}
}
```

## Rules

- A handler signature is `func(ctx context.Context) error`.
- One flat `package handlers`; distinct `NewXHandler` constructors, no sub-packages.
- Decode payloads at the edge (`anduin.DecodeJSON`/`DecodeForm`); a decode failure is `anduin.BadRequest`. Do not re-implement entity validation.
- Decode into a named, exported `XPayload` struct, never an anonymous `struct{…}`: `andurel sync payloads` only emits TypeScript for named `*Payload` types, so the frontend form can import it from `@/types/payloads`.
- Depend on the narrowest thing: a receiver (`domains.Products`) for single-domain CRUD, a service for multi-domain use cases. Never `storage.*` or `domains/internal/queries`.
- Route registration happens in `Register(rt *anduin.Router)`.
- Return domain errors unchanged; the router error handler maps them uniformly.
- Exception: a form that can hit `domains.ErrConflict` (e.g. a taken email) checks `errors.Is(err, domains.ErrConflict)` and re-renders its own page with a field error, as `AuthHandler.RegistrationCreate` does. Otherwise the user lands on the generic `Errors/Conflict` page.
- Construct the handler once in `internal/runtime` (`handlers.NewProductHandler(products, renderer).Register(router)`, with `products := domains.NewProducts(db)`).

## Checklist

- [ ] `NewXHandler` constructor lists every dependency.
- [ ] `Register` binds each route to a method.
- [ ] Payloads are decoded into named `XPayload` structs, not re-validated; validation errors come from the domain.
- [ ] `andurel sync payloads --json` reports the new payload type.
- [ ] Handler methods return domain errors untouched.
- [ ] Wired via `handlers.NewProductHandler(products, renderer).Register(router)` in `internal/runtime`.
- [ ] `andurel inspect handlers --json` lists the new handler.

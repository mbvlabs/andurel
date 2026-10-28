// Package inertia implements the Inertia v3 protocol for anduin and templ.
package inertia

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// Redirect creates a normal redirect. Middleware upgrades unsafe 302 responses
// to 303 and converts fragment redirects to the v3 control response.
func (renderer *Renderer) Redirect(ctx context.Context, location string, status int) error {
	if status < 300 || status > 399 {
		return fmt.Errorf("inertia: redirect status must be 3xx")
	}
	return anduin.Redirect(ctx, location, status)
}

// Location makes an Inertia visit perform a full browser location visit.
func (renderer *Renderer) Location(ctx context.Context, location string) error {
	request, err := requestState(ctx)
	if err != nil {
		return err
	}
	if !request.Inertia {
		return anduin.Redirect(ctx, location, http.StatusFound)
	}
	header := anduin.ResponseWriter(ctx).Header()
	header.Del(HeaderInertia)
	header.Set(HeaderLocation, location)
	anduin.AddVary(header, HeaderInertia)
	return anduin.NoContent(ctx, http.StatusConflict)
}

// FreshRedirect makes an Inertia visit perform a fresh Inertia GET.
func (renderer *Renderer) FreshRedirect(ctx context.Context, location string) error {
	request, err := requestState(ctx)
	if err != nil {
		return err
	}
	if !request.Inertia {
		return anduin.Redirect(ctx, location, http.StatusFound)
	}
	header := anduin.ResponseWriter(ctx).Header()
	header.Del(HeaderInertia)
	header.Set(HeaderRedirect, location)
	anduin.AddVary(header, HeaderInertia)
	return anduin.NoContent(ctx, http.StatusConflict)
}

// Package inertia implements the Inertia v3 protocol for anduin and templ.
package inertia

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// Redirect creates a normal redirect. Middleware upgrades unsafe 302 responses
// to 303 and converts fragment redirects to the v3 control response.
//
// Inertia visits are XHR requests, which browsers refuse to follow across
// origins, so a redirect to another host is answered with the full page visit
// from Location instead.
func (renderer *Renderer) Redirect(ctx context.Context, location string, status int) error {
	if status < 300 || status > 399 {
		return fmt.Errorf("inertia: redirect status must be 3xx")
	}
	if isCrossDomainLocation(anduin.Request(ctx), location) {
		request, err := requestState(ctx)
		if err != nil {
			return err
		}
		if request.Inertia {
			return renderer.Location(ctx, location)
		}
	}
	return anduin.Redirect(ctx, location, status)
}

// isCrossDomainLocation reports whether location names a host other than the
// one the request was addressed to. The comparison is case-insensitive and
// includes the port. Relative paths and unparsable locations are not
// cross-domain.
func isCrossDomainLocation(request *http.Request, location string) bool {
	if request == nil {
		return false
	}
	target, err := url.Parse(strings.TrimSpace(location))
	if err != nil || target.Host == "" {
		return false
	}
	return !strings.EqualFold(target.Host, request.Host)
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

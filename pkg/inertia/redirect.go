// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

// Redirect creates a normal redirect. Middleware upgrades unsafe 302 responses
// to 303 and converts fragment redirects to the v3 control response.
func (renderer *Renderer) Redirect(etx *echo.Context, location string, status int) error {
	if status < 300 || status > 399 {
		return fmt.Errorf("inertia: redirect status must be 3xx")
	}
	return etx.Redirect(status, location)
}

// Location makes an Inertia visit perform a full browser location visit.
func (renderer *Renderer) Location(etx *echo.Context, location string) error {
	request, err := requestState(etx)
	if err != nil {
		return err
	}
	if !request.Inertia {
		return etx.Redirect(http.StatusFound, location)
	}
	header := etx.Response().Header()
	header.Del(HeaderInertia)
	header.Set(HeaderLocation, location)
	appendVary(header, HeaderInertia)
	return etx.NoContent(http.StatusConflict)
}

// FreshRedirect makes an Inertia visit perform a fresh Inertia GET.
func (renderer *Renderer) FreshRedirect(etx *echo.Context, location string) error {
	request, err := requestState(etx)
	if err != nil {
		return err
	}
	if !request.Inertia {
		return etx.Redirect(http.StatusFound, location)
	}
	header := etx.Response().Header()
	header.Del(HeaderInertia)
	header.Set(HeaderRedirect, location)
	appendVary(header, HeaderInertia)
	return etx.NoContent(http.StatusConflict)
}

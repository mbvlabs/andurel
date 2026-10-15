package handlers

import (
	"context"

	"example.com/app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

// Pages serves the application's Inertia pages. Home is the reference page: it
// renders the Welcome component and links to the auth routes.
type Pages struct {
	renderer *inertia.Renderer
}

func NewPages(renderer *inertia.Renderer) Pages {
	return Pages{renderer: renderer}
}

// Register mounts the page routes. HomePage is "/", so it also acts as the
// least-specific match for unmatched GET requests.
func (p Pages) Register(rt *anduin.Router) {
	rt.Get(routes.HomePage.Path(), p.Home)
}

func (p Pages) Home(ctx context.Context) error {
	return p.renderer.Page(ctx, "Welcome", inertia.Props{
		"appName":         "testapp",
		"docsURL":         "https://andurel.com",
		"registrationURL": routes.RegistrationNew.URL(),
		"loginURL":        routes.SessionNew.URL(),
	}).Render()
}

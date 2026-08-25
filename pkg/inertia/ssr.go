// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

import "context"

// SSRRenderer renders a completed page object. It does not own the runtime process.
type SSRRenderer interface {
	Render(context.Context, Page) (*SSRResponse, error)
}

// SSRResponse contains trusted markup from the configured local SSR runtime.
type SSRResponse struct {
	Head []string `json:"head"`
	Body string   `json:"body"`
}

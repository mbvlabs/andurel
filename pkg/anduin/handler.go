package anduin

import (
	"context"
	"net/http"
)

// Handler is an HTTP endpoint expressed as ordinary Go over a context.
// Returning a non-nil error routes to the router's ErrorHandler.
type Handler func(ctx context.Context) error

// Middleware wraps a Handler with cross-cutting behavior.
type Middleware func(Handler) Handler

// ErrorHandler maps a handler error to a response. Returning nil means the
// error was handled (for example, an error page was rendered).
type ErrorHandler func(ctx context.Context, err error) error

// applyMiddleware builds a chain so that the first middleware is the outermost.
func applyMiddleware(handler Handler, middleware []Middleware) Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		if middleware[i] != nil {
			handler = middleware[i](handler)
		}
	}
	return handler
}

// Serve runs handler with request/response transport state built from w and r.
// It is the low-level entry point behind Router.Handler; it is useful when
// embedding an anduin.Handler in another http.Handler and in tests.
func Serve(w http.ResponseWriter, r *http.Request, handler Handler) error {
	ctx := withState(r.Context(), &requestState{request: r, writer: w})
	return handler(ctx)
}

package anduin

import (
	"context"
	"net/http"
	"slices"
)

// Handler is an HTTP endpoint expressed as ordinary Go over a context.
// Returning a non-nil error routes to the router's ErrorHandler.
type Handler func(ctx context.Context) error

// Middleware wraps a Handler with cross-cutting behavior. An error returned by
// a middleware or its next stays visible to the middleware layers that receive
// it, so rollback and error inspection keep working.
//
// Global and host middleware errors render at the failing boundary, before the
// enclosing middleware unwinds. A global or host middleware that needs to
// consume, translate or retry an error must wrap its own layer with
// CatchErrors; otherwise the error page is already committed by the time the
// middleware sees it. Route middleware already runs in an implicit delayed
// scope, so existing route consumers keep working without a change.
type Middleware func(Handler) Handler

// ErrorHandler maps a handler error to a response. Returning nil means the
// error was handled (for example, an error page was rendered).
//
// For a global or host middleware error, ctx is the context passed to the
// failing layer at the moment it failed, and the enclosing middleware's
// resources are still active. For a delayed scope (route middleware or
// CatchErrors), ctx keeps the deepest values recorded from the latest failing
// pass but takes cancellation, deadline and cause from the live request
// context; the scope middleware's own defers have already run. Values a
// middleware creates but never passes to next are not recoverable.
//
// A middleware that returns nil consumes the error below it. Outside a delayed
// scope, use CatchErrors for that behavior. The router never invokes the
// ErrorHandler twice; after a render, retries are blocked with
// ErrResponseCommitted.
type ErrorHandler func(ctx context.Context, err error) error

// applyMiddleware builds a chain so that the first middleware is the outermost.
// boundary, when non-nil, wraps the handler and every middleware level; the
// router uses it to observe the context each layer was called with.
func applyMiddleware(handler Handler, middleware []Middleware, boundary func(Handler) Handler) Handler {
	if boundary != nil {
		handler = boundary(handler)
	}
	for _, m := range slices.Backward(middleware) {
		if m != nil {
			handler = m(handler)
			if boundary != nil {
				handler = boundary(handler)
			}
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

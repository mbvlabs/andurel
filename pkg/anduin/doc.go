// Package anduin is andurel's single HTTP package: a thin layer over net/http
// and http.ServeMux (Go 1.22+ method and wildcard patterns).
//
// A handler is ordinary Go over a context:
//
//	func (h Handler) Index(ctx context.Context) error {
//		page, err := anduin.QueryInt(ctx, "page", 1)
//		if err != nil {
//			return anduin.BadRequest(err)
//		}
//		return anduin.JSON(ctx, http.StatusOK, result[page])
//	}
//
// Request and response access travels on the context. Only transport data —
// the *http.Request, the http.ResponseWriter, and path wildcards — lives
// there; dependencies and database handles never do.
//
// An error returned by a handler or middleware reaches the ErrorHandler once.
// Global and host middleware errors render at the failing boundary while the
// middleware that passed the failing context is still active, so context values,
// cancellation and resources are live and response/cookie writers still observe
// the final status. Route and handler errors render after route middleware
// unwinds through an implicit delayed scope, preserving existing route error
// consumers; their middleware values are retained but route-scope resources may
// already be released. CatchErrors is the explicit delayed scope for global or
// host middleware that consumes, translates or retries errors.
//
// Delayed scopes merge values from the deepest failing pass with the live
// request's cancellation, deadline and cause. A middleware that consumes an
// error by returning nil inside a delayed scope owns the response and prevents
// the ErrorHandler from running; outside one, wrap the consumer with
// CatchErrors first. Errors always propagate unchanged, so rollback and
// errors.Is/As keep working. Rendering is guarded once per request; a retry
// after a committed render returns ErrResponseCommitted. ResponseBuffer lets a
// capturing writer discard a buffered success before the error page and deliver
// the rendered error; an older capture that discards the render is detected
// through the delivery writer and answered with one safe plain 500 instead of a
// second ErrorHandler call.
//
// Router owns registration and dispatch. Server owns a listener. Keeping them
// separate means a test and a production process use the same Router while only
// the process pays for a Server.
package anduin

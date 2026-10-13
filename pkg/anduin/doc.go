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
// Router owns registration and dispatch. Server owns a listener. Keeping them
// separate means a test and a production process use the same Router while only
// the process pays for a Server.
package anduin

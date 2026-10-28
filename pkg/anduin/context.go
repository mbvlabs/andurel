package anduin

import (
	"context"
	"net/http"

	"github.com/mbvlabs/andurel/pkg/routing"
)

// requestState is the mutable per-request transport state stored on the
// request context. A pointer is shared so that middleware which replaces the
// request or the writer is observed by everything downstream — including the
// handler adapters registered with http.ServeMux, which only see the request.
type requestState struct {
	request  *http.Request
	writer   http.ResponseWriter
	host     routing.HostName
	clientIP string
}

type stateKey struct{}

func withState(ctx context.Context, state *requestState) context.Context {
	return context.WithValue(ctx, stateKey{}, state)
}

func stateFrom(ctx context.Context) *requestState {
	if ctx == nil {
		return nil
	}

	state, _ := ctx.Value(stateKey{}).(*requestState)
	return state
}

// RequestHost returns the routing.HostName the router resolved for the
// request. It is the same answer the dispatcher uses: matching is
// case-insensitive, aliases resolve to their HostName, and an unknown
// authority reports routing.HostPrimary. A configured host is reported even
// when it has no registered routes (dispatch then falls back to the primary
// mux). A context without router state also reports routing.HostPrimary.
func RequestHost(ctx context.Context) routing.HostName {
	if state := stateFrom(ctx); state != nil && state.host != "" {
		return state.host
	}
	return routing.HostPrimary
}

// Request returns the *http.Request carried on ctx, or nil when ctx is not an
// anduin request context.
func Request(ctx context.Context) *http.Request {
	if state := stateFrom(ctx); state != nil {
		return state.request
	}
	return nil
}

// ResponseWriter returns the http.ResponseWriter carried on ctx, or nil when
// ctx is not an anduin request context.
func ResponseWriter(ctx context.Context) http.ResponseWriter {
	if state := stateFrom(ctx); state != nil {
		return state.writer
	}
	return nil
}

// ReplaceRequest swaps the request carried on ctx. Middleware uses it to derive
// a new request context that downstream code observes.
func ReplaceRequest(ctx context.Context, request *http.Request) context.Context {
	if request == nil {
		return ctx
	}
	if state := stateFrom(ctx); state != nil {
		state.request = request
		return ctx
	}
	return withState(ctx, &requestState{request: request})
}

// ReplaceResponseWriter swaps the writer carried on ctx.
func ReplaceResponseWriter(ctx context.Context, writer http.ResponseWriter) context.Context {
	if writer == nil {
		return ctx
	}
	if state := stateFrom(ctx); state != nil {
		state.writer = writer
		return ctx
	}
	return withState(ctx, &requestState{writer: writer})
}

// PathValue returns the value of the named path wildcard for the request, or
// the empty string when it is absent.
func PathValue(ctx context.Context, key string) string {
	if request := Request(ctx); request != nil {
		return request.PathValue(key)
	}
	return ""
}

// SetPathValue sets a path wildcard on the request, useful in tests and for
// handlers that synthesize sub-requests.
func SetPathValue(ctx context.Context, key, value string) {
	if request := Request(ctx); request != nil {
		request.SetPathValue(key, value)
	}
}

// RoutePattern returns the http.ServeMux pattern that matched the request, or
// the empty string when no route matched.
func RoutePattern(ctx context.Context) string {
	if request := Request(ctx); request != nil {
		return request.Pattern
	}
	return ""
}

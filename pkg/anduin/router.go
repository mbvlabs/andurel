package anduin

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/mbvlabs/andurel/pkg/routing"
	"github.com/mbvlabs/andurel/pkg/telemetry"
)

// Option configures a Router.
type Option func(*Router)

// WithErrorHandler installs the router-wide ErrorHandler. Without it, a handler
// error becomes a plain-text response carrying its HTTP status.
func WithErrorHandler(handler ErrorHandler) Option {
	return func(rt *Router) {
		if handler != nil {
			rt.onError = handler
		}
	}
}

// WithTelemetry instruments the router's http.Handler with OpenTelemetry and
// installs the Telemetry middleware. It is equivalent to wrapping Handler() in
// telemetry.WrapHandler plus a call to Use(Telemetry(tel)).
func WithTelemetry(tel *telemetry.Telemetry) Option {
	return func(rt *Router) {
		rt.telemetry = tel
		rt.mw = append(rt.mw, Telemetry(tel))
	}
}

// WithHosts installs the host registry used to dispatch requests to named
// hosts. Without it, every request is served by the primary mux.
func WithHosts(hosts *routing.Hosts) Option {
	return func(rt *Router) {
		rt.hostsRegistry = hosts
	}
}

// Router registers handlers and dispatches them over net/http's ServeMux. It
// owns one route table per host; Router itself is not a listener (see Server).
// A test and a production process share the same Router.
type Router struct {
	primary       *hostTable
	hosts         map[routing.HostName]*hostTable
	hostsRegistry *routing.Hosts
	mw            []Middleware
	onError       ErrorHandler
	telemetry     *telemetry.Telemetry

	mu      sync.Mutex
	handler http.Handler
	built   bool
}

// fallbackPolicy says what a non-primary host does with a request none of its
// own routes match.
type fallbackPolicy int

const (
	fallbackUnset fallbackPolicy = iota
	fallbackToPrimary
	fallbackIsolate
)

// hostTable is one host's route table: a ServeMux, the number of routes
// registered on it, its fallback policy and its host middleware. The primary
// table has no policy because it is the fallback target itself.
type hostTable struct {
	name       routing.HostName
	mux        *http.ServeMux
	routes     int
	policy     fallbackPolicy
	middleware []Middleware
}

func newHostTable(name routing.HostName) *hostTable {
	return &hostTable{name: name, mux: http.NewServeMux()}
}

// NewRouter constructs a Router. Options are applied before any route is
// registered.
func NewRouter(opts ...Option) *Router {
	rt := &Router{
		primary: newHostTable(routing.HostPrimary),
		hosts:   make(map[routing.HostName]*hostTable),
		onError: defaultErrorHandler,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(rt)
		}
	}
	return rt
}

// Get registers a GET handler on the primary host. net/http serves HEAD
// requests with GET handlers.
func (rt *Router) Get(pattern string, h Handler, mw ...Middleware) {
	rt.register(rt.primary, http.MethodGet, pattern, h, mw)
}

// Post registers a POST handler on the primary host.
func (rt *Router) Post(pattern string, h Handler, mw ...Middleware) {
	rt.register(rt.primary, http.MethodPost, pattern, h, mw)
}

// Put registers a PUT handler on the primary host.
func (rt *Router) Put(pattern string, h Handler, mw ...Middleware) {
	rt.register(rt.primary, http.MethodPut, pattern, h, mw)
}

// Patch registers a PATCH handler on the primary host.
func (rt *Router) Patch(pattern string, h Handler, mw ...Middleware) {
	rt.register(rt.primary, http.MethodPatch, pattern, h, mw)
}

// Delete registers a DELETE handler on the primary host.
func (rt *Router) Delete(pattern string, h Handler, mw ...Middleware) {
	rt.register(rt.primary, http.MethodDelete, pattern, h, mw)
}

// Use appends global middleware. It must be called before Handler.
func (rt *Router) Use(mw ...Middleware) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.mw = append(rt.mw, mw...)
}

// Handler returns an http.Handler that dispatches on Host, runs the global
// middleware chain, calls the matched anduin.Handler, and routes any returned
// error to the ErrorHandler. The result is cached; call Use and register routes
// before the first call.
//
// Handler panics when a non-primary host has routes but no fallback policy. Call
// Host(name).FallbackToPrimary() or Host(name).Isolate() for every such host.
func (rt *Router) Handler() http.Handler {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.built {
		return rt.handler
	}

	for _, name := range slices.Sorted(maps.Keys(rt.hosts)) {
		table := rt.hosts[name]
		if table.routes > 0 && table.policy == fallbackUnset {
			panic(fmt.Sprintf(
				"anduin: host %q has routes but no fallback policy; call rt.Host(%q).FallbackToPrimary() or .Isolate()",
				name, name,
			))
		}
	}

	handler := http.Handler(http.HandlerFunc(rt.serve))
	if rt.telemetry != nil {
		handler = telemetry.WrapHandler("http", handler, rt.telemetry)
	}
	rt.handler = handler
	rt.built = true
	return rt.handler
}

func (rt *Router) register(table *hostTable, method, pattern string, h Handler, mw []Middleware) {
	if h == nil {
		panic("anduin: nil handler for " + method + " " + pattern)
	}
	table.mux.Handle(method+" "+MuxPattern(pattern), rt.routeHandler(h, mw))
	rt.countRoute(table)
}

// countRoute records one more route on table. Fallback policy checks read the
// count, so it is kept under the router lock.
func (rt *Router) countRoute(table *hostTable) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	table.routes++
}

// routeHandler adapts an anduin.Handler into the http.HandlerFunc that
// http.ServeMux invokes (with path values and Pattern already populated).
func (rt *Router) routeHandler(h Handler, mw []Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if stateFrom(ctx) == nil {
			ctx = withState(ctx, &requestState{})
		}
		ctx = ReplaceRequest(ctx, r)
		ctx = ReplaceResponseWriter(ctx, w)

		handler := applyMiddleware(h, mw)
		if err := handler(ctx); err != nil {
			rt.handleError(ctx, err)
		}
	}
}

// serve is the router's http.Handler entry point. It creates the per-request
// transport state, runs the global middleware chain, and dispatches to the
// route table that serves the request. The host's own middleware runs between
// the global chain and dispatch, and only when the host's own table serves the
// request.
func (rt *Router) serve(w http.ResponseWriter, r *http.Request) {
	host := rt.resolveHost(r.Host)
	ctx := withState(r.Context(), &requestState{request: r, writer: w, host: host})

	base := func(ctx context.Context) error {
		request := Request(ctx)
		if request == nil {
			return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: no request on context"}
		}
		// Re-derive the request so that the final context (state plus every
		// middleware value) is visible to the handler and so that http.ServeMux
		// sets Pattern and path values on the request we can observe.
		request = request.WithContext(ctx)
		ctx = ReplaceRequest(ctx, request)

		table, hostMiddleware := rt.tableFor(host, request)
		dispatch := func(ctx context.Context) error {
			rt.dispatch(ctx, table)
			return nil
		}
		return applyMiddleware(dispatch, hostMiddleware)(ctx)
	}

	handler := applyMiddleware(base, rt.mw)
	if err := handler(ctx); err != nil {
		rt.handleError(ctx, err)
	}
}

// tableFor picks the route table that serves request on host, and returns the
// middleware that table's host runs first. A host with no routes is served by
// the primary table. A FallbackToPrimary host is served by the primary table
// when its own table has no match for the request, and in that case no host
// middleware runs.
func (rt *Router) tableFor(host routing.HostName, request *http.Request) (*hostTable, []Middleware) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if host == routing.HostPrimary {
		return rt.primary, slices.Clone(rt.primary.middleware)
	}
	table, ok := rt.hosts[host]
	if !ok || table.routes == 0 {
		return rt.primary, nil
	}
	if table.policy == fallbackToPrimary {
		if _, pattern := table.mux.Handler(request); pattern == "" {
			return rt.primary, nil
		}
	}
	return table, slices.Clone(table.middleware)
}

// dispatch hands the request to table's ServeMux and turns the mux's own 404
// and 405 responses into router errors.
func (rt *Router) dispatch(ctx context.Context, table *hostTable) {
	request := Request(ctx)
	writer := &muxStatusWriter{ResponseWriter: ResponseWriter(ctx), request: request}
	table.mux.ServeHTTP(writer, request)
	switch writer.status {
	case http.StatusNotFound:
		rt.handleError(ctx, ErrNotFound)
	case http.StatusMethodNotAllowed:
		rt.handleError(ctx, ErrMethodNotAllowed)
	}
}

// resolveHost maps the request authority to a configured HostName, falling back
// to HostPrimary when the registry is absent or the authority is unknown.
func (rt *Router) resolveHost(authority string) routing.HostName {
	if rt.hostsRegistry != nil {
		if name, ok := rt.hostsRegistry.Resolve(authority); ok {
			return name
		}
	}
	return routing.HostPrimary
}

// muxStatusWriter turns the ServeMux's built-in 404 and 405 responses into
// router errors so the ErrorHandler can render the app's pages. ServeMux has no
// error hooks, and a catch-all "/" pattern would also swallow
// method-not-allowed responses, so the generated responses are intercepted
// instead. A matched handler has a non-empty request.Pattern and passes through
// untouched. ServeMux sets Allow before WriteHeader(405); only the status and
// body are intercepted, so the header survives into the ErrorHandler's
// response.
type muxStatusWriter struct {
	http.ResponseWriter
	request     *http.Request
	status      int
	wroteHeader bool
}

func (w *muxStatusWriter) WriteHeader(status int) {
	if !w.wroteHeader && w.request.Pattern == "" &&
		(status == http.StatusNotFound || status == http.StatusMethodNotAllowed) {
		w.status = status
		w.wroteHeader = true
		return
	}
	w.ResponseWriter.WriteHeader(status)
	w.wroteHeader = true
}

func (w *muxStatusWriter) Write(body []byte) (int, error) {
	w.wroteHeader = true
	if w.status != 0 {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *muxStatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush keeps streaming handlers (SSE) working through the wrapper.
func (w *muxStatusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

var (
	_ http.Flusher                              = (*muxStatusWriter)(nil)
	_ interface{ Unwrap() http.ResponseWriter } = (*muxStatusWriter)(nil)
)

// handleError routes err to the ErrorHandler, falling back to a plain response
// when the handler declines it.
func (rt *Router) handleError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	if rt.onError != nil {
		if unhandled := rt.onError(ctx, err); unhandled != nil {
			defaultErrorHandler(ctx, unhandled)
		}
		return
	}
	defaultErrorHandler(ctx, err)
}

// MuxPattern converts an andurel/routing path into an http.ServeMux pattern.
// It rewrites ":name" segments to "{name}", a trailing "*" to "{path...}", and
// leaves existing "{...}" wildcards untouched.
func MuxPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "/"
	}
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		switch {
		case strings.HasPrefix(segment, ":") && len(segment) > 1:
			segments[i] = "{" + segment[1:] + "}"
		case segment == "*":
			segments[i] = "{" + routing.WildcardParam + "...}"
		}
	}
	return strings.Join(segments, "/")
}

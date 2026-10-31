package anduin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mbvlabs/andurel/pkg/routing"
)

// RouteValue is a route declared with pkg/routing (routing.NewSimpleRoute and
// the typed route constructors). Route reads the host the route is bound to and
// the path it registers.
type RouteValue interface {
	Host() routing.HostName
	Pattern() string
}

// Host returns the named host's route table. Routes registered on the
// primary host (HostPrimary, or the empty name) go to the primary table that
// Get and friends use.
func (rt *Router) Host(name routing.HostName) *Host {
	name = normalizeHost(name)
	if name == routing.HostPrimary {
		return &Host{router: rt, table: rt.primary, name: name}
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()
	table, ok := rt.hosts[name]
	if !ok {
		table = newHostTable(name)
		rt.hosts[name] = table
	}
	return &Host{router: rt, table: table, name: name}
}

// Route returns a registrar for r on the host r declares. It is the same as
// Host(r.Host()).Route(r), so the route value decides the domain it serves on.
func (rt *Router) Route(r RouteValue) RouteRegistrar {
	return rt.Host(r.Host()).Route(r)
}

// Host is a named virtual host: its own route table, with the same
// registration surface as Router plus the host's fallback policy, middleware
// and mounts.
type Host struct {
	router *Router
	table  *hostTable
	name   routing.HostName
}

// Name returns the HostName this Host serves.
func (h *Host) Name() routing.HostName { return h.name }

// Route returns a registrar for r on this host. It panics when r declares a
// different host, because the route would otherwise be served on a domain its
// declaration does not name.
func (h *Host) Route(r RouteValue) RouteRegistrar {
	if r == nil {
		panic("anduin: nil route for host " + string(h.name))
	}
	if declared := normalizeHost(r.Host()); declared != h.name {
		panic(fmt.Sprintf(
			"anduin: route %q declares host %q but was registered on host %q",
			r.Pattern(), declared, h.name,
		))
	}
	return RouteRegistrar{host: h, pattern: r.Pattern()}
}

// FallbackToPrimary makes the host check its own routes first and use the
// primary table only when none of them match the request. Assets and robots.txt
// registered on the primary host then work on this host without being
// registered twice. Requests served by the primary table do not run this
// host's middleware.
func (h *Host) FallbackToPrimary() {
	h.setPolicy(fallbackToPrimary)
}

// Isolate makes unmatched requests on this host 404 without consulting the
// primary table.
func (h *Host) Isolate() {
	h.setPolicy(fallbackIsolate)
}

func (h *Host) setPolicy(policy fallbackPolicy) {
	h.router.mu.Lock()
	defer h.router.mu.Unlock()
	if h.table.policy != fallbackUnset && h.table.policy != policy {
		panic(fmt.Sprintf(
			"anduin: host %q already has a fallback policy; call FallbackToPrimary or Isolate, not both",
			h.name,
		))
	}
	h.table.policy = policy
}

// Use appends middleware for requests this host serves from its own table. It
// runs after the global middleware and before per-route middleware.
func (h *Host) Use(mw ...Middleware) {
	h.router.mu.Lock()
	defer h.router.mu.Unlock()
	for _, m := range mw {
		if m != nil {
			h.table.middleware = append(h.table.middleware, m)
		}
	}
}

// Mount serves handler for every method on prefix and on prefix/ and everything
// below it. The handler sees the full request path. A prefix of "" or "/"
// mounts the handler on the whole host.
func (h *Host) Mount(prefix string, handler http.Handler) {
	if handler == nil {
		panic("anduin: nil handler for mount " + prefix)
	}

	trimmed := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	patterns := []string{"/"}
	if trimmed != "" {
		if !strings.HasPrefix(trimmed, "/") {
			trimmed = "/" + trimmed
		}
		patterns = []string{trimmed, trimmed + "/*"}
	}
	for _, pattern := range patterns {
		h.table.mux.Handle(MuxPattern(pattern), handler)
	}
	h.router.countRoute(h.table)
}

// Get registers a GET handler on the host. net/http serves HEAD requests with
// GET handlers.
func (h *Host) Get(pattern string, handler Handler, mw ...Middleware) {
	h.router.register(h.table, http.MethodGet, pattern, handler, mw)
}

// Post registers a POST handler on the host.
func (h *Host) Post(pattern string, handler Handler, mw ...Middleware) {
	h.router.register(h.table, http.MethodPost, pattern, handler, mw)
}

// Put registers a PUT handler on the host.
func (h *Host) Put(pattern string, handler Handler, mw ...Middleware) {
	h.router.register(h.table, http.MethodPut, pattern, handler, mw)
}

// Patch registers a PATCH handler on the host.
func (h *Host) Patch(pattern string, handler Handler, mw ...Middleware) {
	h.router.register(h.table, http.MethodPatch, pattern, handler, mw)
}

// Delete registers a DELETE handler on the host.
func (h *Host) Delete(pattern string, handler Handler, mw ...Middleware) {
	h.router.register(h.table, http.MethodDelete, pattern, handler, mw)
}

// RouteRegistrar registers handlers for one pkg/routing route on the host the
// route declares. Obtain it from Router.Route or Host.Route.
type RouteRegistrar struct {
	host    *Host
	pattern string
}

// Get registers a GET handler for the route. net/http serves HEAD requests with
// GET handlers.
func (r RouteRegistrar) Get(handler Handler, mw ...Middleware) {
	r.host.router.register(r.host.table, http.MethodGet, r.pattern, handler, mw)
}

// Post registers a POST handler for the route.
func (r RouteRegistrar) Post(handler Handler, mw ...Middleware) {
	r.host.router.register(r.host.table, http.MethodPost, r.pattern, handler, mw)
}

// Put registers a PUT handler for the route.
func (r RouteRegistrar) Put(handler Handler, mw ...Middleware) {
	r.host.router.register(r.host.table, http.MethodPut, r.pattern, handler, mw)
}

// Patch registers a PATCH handler for the route.
func (r RouteRegistrar) Patch(handler Handler, mw ...Middleware) {
	r.host.router.register(r.host.table, http.MethodPatch, r.pattern, handler, mw)
}

// Delete registers a DELETE handler for the route.
func (r RouteRegistrar) Delete(handler Handler, mw ...Middleware) {
	r.host.router.register(r.host.table, http.MethodDelete, r.pattern, handler, mw)
}

func normalizeHost(name routing.HostName) routing.HostName {
	if name == "" {
		return routing.HostPrimary
	}
	return name
}

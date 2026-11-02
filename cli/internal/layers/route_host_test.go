package layers

import (
	"strings"
	"testing"
)

// TestRouteHostRuleSeesRoutesDeclaredInOtherPackages loads a real project so
// the analyzer reads the route's host from the routes package, not from the
// handler package that registers it.
func TestRouteHostRuleSeesRoutesDeclaredInOtherPackages(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "go.mod", `module app

go 1.21

require (
	github.com/mbvlabs/andurel/pkg/anduin v0.0.0
	github.com/mbvlabs/andurel/pkg/routing v0.0.0
)

replace github.com/mbvlabs/andurel/pkg/anduin => ./stubs/anduin

replace github.com/mbvlabs/andurel/pkg/routing => ./stubs/routing
`)
	writeProjectFile(t, root, "stubs/routing/go.mod", "module github.com/mbvlabs/andurel/pkg/routing\n\ngo 1.21\n")
	writeProjectFile(t, root, "stubs/routing/routing.go", `package routing

type HostName string

const HostPrimary HostName = "primary"

type Route struct {
	path string
	host HostName
}

type RouteSetupOption func(*Route)

func NewSimpleRoute(path, name, prefix string, opts ...RouteSetupOption) Route {
	route := Route{path: path}
	for _, opt := range opts {
		opt(&route)
	}
	return route
}

func Host(name HostName) RouteSetupOption {
	return func(route *Route) { route.host = name }
}

func (r Route) Pattern() string { return r.path }
`)
	writeProjectFile(t, root, "stubs/anduin/go.mod", `module github.com/mbvlabs/andurel/pkg/anduin

go 1.21

require github.com/mbvlabs/andurel/pkg/routing v0.0.0

replace github.com/mbvlabs/andurel/pkg/routing => ../routing
`)
	writeProjectFile(t, root, "stubs/anduin/anduin.go", `package anduin

import "github.com/mbvlabs/andurel/pkg/routing"

type Handler func() error

type Router struct{}

func (rt *Router) Get(pattern string, h Handler) {}

func (rt *Router) Route(r routing.Route) RouteRegistrar { return RouteRegistrar{} }

type RouteRegistrar struct{}

func (r RouteRegistrar) Get(h Handler) {}
`)
	writeProjectFile(t, root, "routes/routes.go", `package routes

import "github.com/mbvlabs/andurel/pkg/routing"

const HostAdmin routing.HostName = "admin"

var AdminDashboard = routing.NewSimpleRoute("/dashboard", "admin.dashboard", "", routing.Host(HostAdmin))
`)
	writeProjectFile(t, root, "handlers/handlers.go", `package handlers

import (
	"app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

func handler() error { return nil }

func Register(rt *anduin.Router) {
	rt.Get(routes.AdminDashboard.Pattern(), handler)
	rt.Route(routes.AdminDashboard).Get(handler)
}
`)

	violations, err := AnalyzePatternsRules(root, nil, []string{"route-host"})
	if err != nil {
		t.Fatalf("AnalyzePatternsRules: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %#v, want exactly one route-host violation", violations)
	}
	violation := violations[0]
	if violation.Rule != "route-host" || violation.File != "handlers/handlers.go" || violation.Line != 12 {
		t.Fatalf("violation = %#v, want route-host at handlers/handlers.go:12", violation)
	}
	if !strings.Contains(violation.Message, `"admin"`) || !strings.Contains(violation.Message, "rt.Route(routes.AdminDashboard).Get") {
		t.Fatalf("message = %q, want the declared host and the rt.Route fix", violation.Message)
	}
}

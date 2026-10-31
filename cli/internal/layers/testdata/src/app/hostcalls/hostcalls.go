package hostcalls

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
)

const HostAdmin routing.HostName = "admin"

var AdminDashboard = routing.NewSimpleRoute("/dashboard", "admin.dashboard", "", routing.Host(HostAdmin))

var Home = routing.NewSimpleRoute("/", "home", "")

func handler(ctx context.Context) error { return nil }

type store struct{}

func (store) Get(pattern string, h anduin.Handler) {}

func Register(rt *anduin.Router) {
	admin := rt.Host(HostAdmin)

	rt.Get(AdminDashboard.Pattern(), handler)    // want "route-host"
	rt.Post(AdminDashboard.Pattern(), handler)   // want "route-host"
	admin.Get(AdminDashboard.Pattern(), handler) // want "route-host"

	admin.Route(AdminDashboard).Get(handler)
	rt.Route(AdminDashboard).Get(handler)
	rt.Get(Home.Pattern(), handler)
	rt.Get("/adhoc", handler)
}

// Other types with a Get method are not route registrars.
func Unrelated(s store) {
	s.Get(AdminDashboard.Pattern(), handler)
}

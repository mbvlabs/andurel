package routing

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

func (r Route) Host() HostName {
	if r.host == "" {
		return HostPrimary
	}

	return r.host
}

func ConfigureHosts(any) error { return nil }

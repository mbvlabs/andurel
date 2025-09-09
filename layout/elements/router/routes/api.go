package routes

import "net/http"

const (
	APIRoutePrefix = "/api"
	apiNamePrefix  = "api"
)

var apiRoutes = []Route{
	Health,
}

var Health = Route{
	Name:         apiNamePrefix + ".health",
	Path:         APIRoutePrefix + "/health",
	Method:       http.MethodGet,
	Handler:      "API",
	HandleMethod: "Health",
}

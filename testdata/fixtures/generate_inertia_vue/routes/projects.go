package routes

import (
	"github.com/mbvlabs/andurel/pkg/routing"
)

const ProjectPrefix = "/projects"

var ProjectIndex = routing.NewSimpleRoute(
	"",
	"projects.index",
	ProjectPrefix,
	routing.InertiaRoute(),
)

var ProjectShow = routing.NewRouteWithUUIDID(
	"/:id",
	"projects.show",
	ProjectPrefix,
	routing.InertiaRoute(),
)

var ProjectCreate = routing.NewSimpleRoute(
	"",
	"projects.create",
	ProjectPrefix,
	routing.InertiaRoute(),
)

var ProjectUpdate = routing.NewRouteWithUUIDID(
	"/:id",
	"projects.update",
	ProjectPrefix,
	routing.InertiaRoute(),
)

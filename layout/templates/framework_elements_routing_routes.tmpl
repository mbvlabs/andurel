// Package routing provides implementations of different kinds of routes.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package routing

import (
	"strings"

	"github.com/google/uuid"
)

func configureName(name, prefix string) string {
	if prefix == "" {
		return name
	}

	return prefix + "." + name
}

func configurePath(path, prefix string) string {
	if prefix == "" {
		return path
	}

	if !strings.Contains(prefix, "/") {
		return prefix + "/" + path
	}

	return prefix + path
}

type Route struct {
	name   string
	path   string
	prefix string
}

var _ SimpleRoute = (*Route)(nil)

// NewSimpleRoute creates a base route that takes no parameters
func NewSimpleRoute(path, name, prefix string) Route {
	return Route{name, path, prefix}
}

func (r Route ) URL() string {
	return configurePath(r.path, r.prefix)
}

func (r Route) Name() string {
	return configureName(r.name, r.prefix)
}

func (r Route) Path() string {
	return configurePath(r.path, r.prefix)
}

type RouteWithID Route

var _ IDRoute = (*RouteWithID)(nil)

// NewRouteWithID creates an id route that takes a uuid as a parameter
func NewRouteWithID(path, name, prefix string) RouteWithID {
	return RouteWithID{name, path, prefix}
}

func (r RouteWithID) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithID) Path() string {
	return configurePath(r.path, r.prefix)
}

func (r RouteWithID) URL(id uuid.UUID) string {
	return strings.Replace(r.Path(), ":id", id.String(), 1)
}

type RouteWithIDs Route

var _ IDsRoute = (*RouteWithIDs)(nil)

// NewRouteWithMultipleIDs creates a route that takes a map[string]uuid as parameters
func NewRouteWithMultipleIDs(path, name, prefix string) RouteWithIDs {
	return RouteWithIDs{name, path, prefix}
}

func (r RouteWithIDs) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithIDs) Path() string {
	return configurePath(r.path, r.prefix)
}

func (r RouteWithIDs) URL(ids map[string]uuid.UUID) string {
	route := r.Path()
	for param, id := range ids {
		route = strings.Replace(route, ":"+param, id.String(), 1)
	}

	return route
}

type RouteWithSlug Route

var _ ParamRoute = (*RouteWithSlug)(nil)

func NewRouteWithSlug(path, name, prefix string) RouteWithSlug {
	return RouteWithSlug{name, path, prefix}
}

func (r RouteWithSlug) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithSlug) Path() string {
	return configurePath(r.path, r.prefix)
}

func (r RouteWithSlug) URL(slug string) string {
	return strings.Replace(r.Path(), ":slug", slug, 1)
}

type RouteWithToken Route

var _ ParamRoute = (*RouteWithToken)(nil)

func NewRouteWithToken(path, name, prefix string) RouteWithToken {
	return RouteWithToken{name, path, prefix}
}

func (r RouteWithToken) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithToken) Path() string {
	return configurePath(r.path, r.prefix)
}

func (r RouteWithToken) URL(token string) string {
	return strings.Replace(r.Path(), ":token", token, 1)
}

type RouteWithFile Route

var _ ParamRoute = (*RouteWithFile)(nil)

func NewRouteWithFile(path, name, prefix string) RouteWithFile {
	return RouteWithFile{name, path, prefix}
}

func (r RouteWithFile) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithFile) Path() string {
	return configurePath(r.path, r.prefix)
}

func (r RouteWithFile) URL(file string) string {
	return strings.Replace(r.Path(), ":file", file, 1)
}

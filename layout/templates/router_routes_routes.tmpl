// Package routes provides the application route definitions.
package routes

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type routeMiddleware interface {
	handler() func(next echo.HandlerFunc) echo.HandlerFunc
}

type middlewareHandler struct {
	fn func(next echo.HandlerFunc) echo.HandlerFunc
}

func (m middlewareHandler) handler() func(next echo.HandlerFunc) echo.HandlerFunc {
	return m.fn
}

func newMiddleware(fn func(next echo.HandlerFunc) echo.HandlerFunc) routeMiddleware {
	return middlewareHandler{fn: fn}
}

type Route struct {
	Name             string
	Path             string
	Controller       string
	ControllerMethod string
	Method           string
	IncludeInSitemap bool
	Middleware       []routeMiddleware
}

type RouteWithID Route

func (r RouteWithID) URL(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

type RouteWithSlug Route

func (r RouteWithSlug) URL(slug string) string {
	return strings.Replace(r.Path, ":slug", slug, 1)
}

type RouteWithToken Route

func (r RouteWithToken) URL(token string) string {
	return strings.Replace(r.Path, ":token", token, 1)
}

var Registry = []Route{}

type routeBuilder Route

func newRouteBuilder(name string) routeBuilder {
	return routeBuilder{
		Name:   name,
		Method: http.MethodGet,
	}
}

func (r routeBuilder) SetNamePrefix(prefix string) routeBuilder {
	r.Name = prefix + "." + r.Name
	return r
}

func (r routeBuilder) SetPath(path string) routeBuilder {
	r.Path = path
	return r
}

func (r routeBuilder) SetMethod(method string) routeBuilder {
	r.Method = method
	return r
}

func (r routeBuilder) SetCtrl(ctrlName, ctrlMethod string) routeBuilder {
	r.Controller = ctrlName
	r.ControllerMethod = ctrlMethod

	return r
}

func (r routeBuilder) WithMiddleware(mw ...routeMiddleware) routeBuilder {
	r.Middleware = append(r.Middleware, mw...)
	return r
}

func (r routeBuilder) WithSitemap() routeBuilder {
	r.IncludeInSitemap = true
	return r
}

func (r routeBuilder) Register() Route {
	Registry = append(Registry, Route(r))

	return Route(r)
}

func (r routeBuilder) RegisterWithID() RouteWithID {
	Registry = append(Registry, Route(r))

	return RouteWithID(r)
}

func (r routeBuilder) RegisterWithSlug() RouteWithSlug {
	Registry = append(Registry, Route(r))

	return RouteWithSlug(r)
}

func (r routeBuilder) RegisterWithToken() RouteWithToken {
	Registry = append(Registry, Route(r))

	return RouteWithToken(r)
}

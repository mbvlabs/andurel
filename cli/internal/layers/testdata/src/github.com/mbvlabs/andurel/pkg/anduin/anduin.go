package anduin

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/routing"
)

func Query(ctx context.Context, key string) string { return "" }

type Handler func(ctx context.Context) error

type Middleware func(Handler) Handler

type Router struct{}

func (rt *Router) Get(pattern string, h Handler, mw ...Middleware)    {}
func (rt *Router) Post(pattern string, h Handler, mw ...Middleware)   {}
func (rt *Router) Put(pattern string, h Handler, mw ...Middleware)    {}
func (rt *Router) Patch(pattern string, h Handler, mw ...Middleware)  {}
func (rt *Router) Delete(pattern string, h Handler, mw ...Middleware) {}

func (rt *Router) Route(r routing.Route) RouteRegistrar { return RouteRegistrar{} }

func (rt *Router) Host(name routing.HostName) *Host { return &Host{} }

type Host struct{}

func (h *Host) Get(pattern string, handler Handler, mw ...Middleware)    {}
func (h *Host) Post(pattern string, handler Handler, mw ...Middleware)   {}
func (h *Host) Put(pattern string, handler Handler, mw ...Middleware)    {}
func (h *Host) Patch(pattern string, handler Handler, mw ...Middleware)  {}
func (h *Host) Delete(pattern string, handler Handler, mw ...Middleware) {}

func (h *Host) Route(r routing.Route) RouteRegistrar { return RouteRegistrar{} }

type RouteRegistrar struct{}

func (r RouteRegistrar) Get(handler Handler, mw ...Middleware)    {}
func (r RouteRegistrar) Post(handler Handler, mw ...Middleware)   {}
func (r RouteRegistrar) Put(handler Handler, mw ...Middleware)    {}
func (r RouteRegistrar) Patch(handler Handler, mw ...Middleware)  {}
func (r RouteRegistrar) Delete(handler Handler, mw ...Middleware) {}

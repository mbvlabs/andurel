package routes

import "github.com/labstack/echo/v4"

type Route struct {
	Name             string
	Path             string
	Controller       string
	ControllerMethod string
	Method           string
	Middleware       []func(next echo.HandlerFunc) echo.HandlerFunc
}

var BuildRoutes = func() []Route {
	var r []Route

	r = append(
		r,
		assetRoutes...,
	)

	r = append(
		r,
		pageRoutes...,
	)

	r = append(
		r,
		apiRoutes...,
	)

	r = append(
		r,
		UserRoutes...,
	)

	return r
}()

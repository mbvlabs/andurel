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

var BuildRoutes = []Route{
	Health,

	Robots,
	Sitemap,
	Stylesheet,
	Scripts,
	Script,

	HomePage,

	UserIndex,
	UserShow.Route,
	UserNew,
	UserCreate,
	UserEdit.Route,
	UserUpdate.Route,
	UserDestroy.Route,
}

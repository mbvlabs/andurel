package router

import (
	"net/http"

	"github.com/example/testapp/controllers"
	"github.com/example/testapp/router/routes"

	"github.com/labstack/echo/v4"
)

func registrar(handler *echo.Echo, ctrls controllers.Controllers) {
	handler.Add(
		http.MethodGet, routes.Health.URL(), ctrls.API.Health,
	).Name = routes.Health.Name()

	handler.Add(
		http.MethodGet, routes.Robots.URL(), ctrls.Assets.Robots,
	).Name = routes.Robots.Name()

	handler.Add(
		http.MethodGet, routes.Sitemap.URL(), ctrls.Assets.Sitemap,
	).Name = routes.Sitemap.Name()

	handler.Add(
		http.MethodGet, routes.HomePage.URL(), ctrls.Pages.Home,
	).Name = routes.HomePage.Name()

	handler.RouteNotFound("/*", ctrls.Pages.NotFound)
}

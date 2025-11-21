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

	registerUsersRoutes(handler, ctrls)

	registerProductsRoutes(handler, ctrls)
}

func registerUsersRoutes(handler *echo.Echo, ctrls controllers.Controllers) {
	handler.Add(
		http.MethodGet, routes.UserIndex.URL(), ctrls.Users.Index,
	).Name = routes.UserIndex.Name()

	handler.Add(
		http.MethodGet, routes.UserShow.URL(), ctrls.Users.Show,
	).Name = routes.UserShow.Name()

	handler.Add(
		http.MethodGet, routes.UserNew.URL(), ctrls.Users.New,
	).Name = routes.UserNew.Name()

	handler.Add(
		http.MethodPost, routes.UserCreate.URL(), ctrls.Users.Create,
	).Name = routes.UserCreate.Name()

	handler.Add(
		http.MethodGet, routes.UserEdit.URL(), ctrls.Users.Edit,
	).Name = routes.UserEdit.Name()

	handler.Add(
		http.MethodPut, routes.UserUpdate.URL(), ctrls.Users.Update,
	).Name = routes.UserUpdate.Name()

	handler.Add(
		http.MethodDelete, routes.UserDestroy.URL(), ctrls.Users.Destroy,
	).Name = routes.UserDestroy.Name()
}

func registerProductsRoutes(handler *echo.Echo, ctrls controllers.Controllers) {
	handler.Add(
		http.MethodGet, routes.ProductIndex.URL(), ctrls.Products.Index,
	).Name = routes.ProductIndex.Name()

	handler.Add(
		http.MethodGet, routes.ProductShow.URL(), ctrls.Products.Show,
	).Name = routes.ProductShow.Name()

	handler.Add(
		http.MethodGet, routes.ProductNew.URL(), ctrls.Products.New,
	).Name = routes.ProductNew.Name()

	handler.Add(
		http.MethodPost, routes.ProductCreate.URL(), ctrls.Products.Create,
	).Name = routes.ProductCreate.Name()

	handler.Add(
		http.MethodGet, routes.ProductEdit.URL(), ctrls.Products.Edit,
	).Name = routes.ProductEdit.Name()

	handler.Add(
		http.MethodPut, routes.ProductUpdate.URL(), ctrls.Products.Update,
	).Name = routes.ProductUpdate.Name()

	handler.Add(
		http.MethodDelete, routes.ProductDestroy.URL(), ctrls.Products.Destroy,
	).Name = routes.ProductDestroy.Name()
}

package router

import (
	"net/http"

	"github.com/example/testapp/controllers"
	"github.com/example/testapp/router/routes"

	"github.com/labstack/echo/v4"
)

func registerCoreRoutes(handler *echo.Echo, ctrls controllers.Controllers) {
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
}

func registerUsersRoutes(handler *echo.Echo, ctrls controllers.Controllers) {
	handler.Add(
		http.MethodGet, routes.UserIndex.Path(), ctrls.Users.Index,
	).Name = routes.UserIndex.Name()

	handler.Add(
		http.MethodGet, routes.UserShow.Path(), ctrls.Users.Show,
	).Name = routes.UserShow.Name()

	handler.Add(
		http.MethodGet, routes.UserNew.Path(), ctrls.Users.New,
	).Name = routes.UserNew.Name()

	handler.Add(
		http.MethodPost, routes.UserCreate.Path(), ctrls.Users.Create,
	).Name = routes.UserCreate.Name()

	handler.Add(
		http.MethodGet, routes.UserEdit.Path(), ctrls.Users.Edit,
	).Name = routes.UserEdit.Name()

	handler.Add(
		http.MethodPut, routes.UserUpdate.Path(), ctrls.Users.Update,
	).Name = routes.UserUpdate.Name()

	handler.Add(
		http.MethodDelete, routes.UserDestroy.Path(), ctrls.Users.Destroy,
	).Name = routes.UserDestroy.Name()
}

func registerProductsRoutes(handler *echo.Echo, ctrls controllers.Controllers) {
	handler.Add(
		http.MethodGet, routes.ProductIndex.Path(), ctrls.Products.Index,
	).Name = routes.ProductIndex.Name()

	handler.Add(
		http.MethodGet, routes.ProductShow.Path(), ctrls.Products.Show,
	).Name = routes.ProductShow.Name()

	handler.Add(
		http.MethodGet, routes.ProductNew.Path(), ctrls.Products.New,
	).Name = routes.ProductNew.Name()

	handler.Add(
		http.MethodPost, routes.ProductCreate.Path(), ctrls.Products.Create,
	).Name = routes.ProductCreate.Name()

	handler.Add(
		http.MethodGet, routes.ProductEdit.Path(), ctrls.Products.Edit,
	).Name = routes.ProductEdit.Name()

	handler.Add(
		http.MethodPut, routes.ProductUpdate.Path(), ctrls.Products.Update,
	).Name = routes.ProductUpdate.Name()

	handler.Add(
		http.MethodDelete, routes.ProductDestroy.Path(), ctrls.Products.Destroy,
	).Name = routes.ProductDestroy.Name()
}

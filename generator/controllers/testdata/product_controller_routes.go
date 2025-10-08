package routes

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	productsRoutePrefix = "/products"
	productsNamePrefix  = "products"
)

var ProductRoutes = []Route{
	ProductIndex,
	ProductShow.Route,
	ProductNew,
	ProductCreate,
	ProductEdit.Route,
	ProductUpdate.Route,
	ProductDestroy.Route,
}

var ProductIndex = Route{
	Name:             productsNamePrefix + ".index",
	Path:             productsRoutePrefix,
	Method:           http.MethodGet,
	Controller:       "Products",
	ControllerMethod: "Index",
}

var ProductShow = productsShow{
	Route: Route{
		Name:             productsNamePrefix + ".show",
		Path:             productsRoutePrefix + "/:id",
		Method:           http.MethodGet,
		Controller:       "Products",
		ControllerMethod: "Show",
	},
}

type productsShow struct {
	Route
}

func (r productsShow) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var ProductNew = Route{
	Name:             productsNamePrefix + ".new",
	Path:             productsRoutePrefix + "/new",
	Method:           http.MethodGet,
	Controller:       "Products",
	ControllerMethod: "New",
}

var ProductCreate = Route{
	Name:             productsNamePrefix + ".create",
	Path:             productsRoutePrefix,
	Method:           http.MethodPost,
	Controller:       "Products",
	ControllerMethod: "Create",
}

var ProductEdit = productsEdit{
	Route: Route{
		Name:             productsNamePrefix + ".edit",
		Path:             productsRoutePrefix + "/:id/edit",
		Method:           http.MethodGet,
		Controller:       "Products",
		ControllerMethod: "Edit",
	},
}

type productsEdit struct {
	Route
}

func (r productsEdit) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var ProductUpdate = productsUpdate{
	Route: Route{
		Name:             productsNamePrefix + ".update",
		Path:             productsRoutePrefix + "/:id",
		Method:           http.MethodPut,
		Controller:       "Products",
		ControllerMethod: "Update",
	},
}

type productsUpdate struct {
	Route
}

func (r productsUpdate) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var ProductDestroy = productsDestroy{
	Route: Route{
		Name:             productsNamePrefix + ".destroy",
		Path:             productsRoutePrefix + "/:id",
		Method:           http.MethodDelete,
		Controller:       "Products",
		ControllerMethod: "Destroy",
	},
}

type productsDestroy struct {
	Route
}

func (r productsDestroy) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

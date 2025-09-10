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
	Name:         productsNamePrefix + ".index",
	Path:         productsRoutePrefix,
	Method:       http.MethodGet,
	Handler:      "Products",
	HandleMethod: "Index",
}

var ProductShow = productsShow{
	Route: Route{
		Name:         productsNamePrefix + ".show",
		Path:         productsRoutePrefix + "/:id",
		Method:       http.MethodGet,
		Handler:      "Products",
		HandleMethod: "Show",
	},
}

type productsShow struct {
	Route
}

func (r productsShow) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var ProductNew = Route{
	Name:         productsNamePrefix + ".new",
	Path:         productsRoutePrefix + "/new",
	Method:       http.MethodGet,
	Handler:      "Products",
	HandleMethod: "New",
}

var ProductCreate = Route{
	Name:         productsNamePrefix + ".create",
	Path:         productsRoutePrefix,
	Method:       http.MethodPost,
	Handler:      "Products",
	HandleMethod: "Create",
}

var ProductEdit = productsEdit{
	Route: Route{
		Name:         productsNamePrefix + ".edit",
		Path:         productsRoutePrefix + "/:id/edit",
		Method:       http.MethodGet,
		Handler:      "Products",
		HandleMethod: "Edit",
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
		Name:         productsNamePrefix + ".update",
		Path:         productsRoutePrefix + "/:id",
		Method:       http.MethodPut,
		Handler:      "Products",
		HandleMethod: "Update",
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
		Name:         productsNamePrefix + ".destroy",
		Path:         productsRoutePrefix + "/:id",
		Method:       http.MethodDelete,
		Handler:      "Products",
		HandleMethod: "Destroy",
	},
}

type productsDestroy struct {
	Route
}

func (r productsDestroy) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

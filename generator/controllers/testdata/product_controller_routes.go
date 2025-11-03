package routes

import "net/http"

const (
	productsRoutePrefix = "/products"
	productsNamePrefix  = "products"
)

var productsGroup = NewRouteGroup(productsNamePrefix, productsRoutePrefix)

var ProductIndex = productsGroup.Route("index").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Index").
	Register()

var ProductShow = productsGroup.Route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Show").
	RegisterWithID()

var ProductNew = productsGroup.Route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "New").
	Register()

var ProductCreate = productsGroup.Route("create").
	SetMethod(http.MethodPost).
	SetCtrl("Products", "Create").
	Register()

var ProductEdit = productsGroup.Route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Edit").
	RegisterWithID()

var ProductUpdate = productsGroup.Route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("Products", "Update").
	RegisterWithID()

var ProductDestroy = productsGroup.Route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("Products", "Destroy").
	RegisterWithID()

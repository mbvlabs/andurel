package routes

import "net/http"

const (
	productsRoutePrefix = "/products"
	productsNamePrefix  = "products"
)

var productsGroup = newRouteGroup(productsNamePrefix, productsRoutePrefix)

var ProductIndex = productsGroup.route("index").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Index").
	Register()

var ProductShow = productsGroup.route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Show").
	RegisterWithID()

var ProductNew = productsGroup.route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "New").
	Register()

var ProductCreate = productsGroup.route("create").
	SetMethod(http.MethodPost).
	SetCtrl("Products", "Create").
	Register()

var ProductEdit = productsGroup.route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Edit").
	RegisterWithID()

var ProductUpdate = productsGroup.route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("Products", "Update").
	RegisterWithID()

var ProductDestroy = productsGroup.route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("Products", "Destroy").
	RegisterWithID()

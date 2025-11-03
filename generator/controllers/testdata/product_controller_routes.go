package routes

import "net/http"

const (
	productsRoutePrefix = "/products"
	productsNamePrefix  = "products"
)

var productsGroup = NewRouteGroup(productsNamePrefix, productsRoutePrefix)

// Index: GET /products
var ProductIndex = productsGroup.Route("index").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Index").
	Register()

// Show: GET /products/:id
var ProductShow = productsGroup.Route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Show").
	RegisterWithID()

// New: GET /products/new
var ProductNew = productsGroup.Route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "New").
	Register()

// Create: POST /products
var ProductCreate = productsGroup.Route("create").
	SetMethod(http.MethodPost).
	SetCtrl("Products", "Create").
	Register()

// Edit: GET /products/:id/edit
var ProductEdit = productsGroup.Route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("Products", "Edit").
	RegisterWithID()

// Update: PUT /products/:id
var ProductUpdate = productsGroup.Route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("Products", "Update").
	RegisterWithID()

// Destroy: DELETE /products/:id
var ProductDestroy = productsGroup.Route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("Products", "Destroy").
	RegisterWithID()

package routes

import "net/http"

const (
	usersRoutePrefix = "/users"
	usersNamePrefix  = "users"
)

var usersGroup = NewRouteGroup(usersNamePrefix, usersRoutePrefix)

// Index: GET /users
var UserIndex = usersGroup.Route("index").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Index").
	Register()

// Show: GET /users/:id
var UserShow = usersGroup.Route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Show").
	RegisterWithID()

// New: GET /users/new
var UserNew = usersGroup.Route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "New").
	Register()

// Create: POST /users
var UserCreate = usersGroup.Route("create").
	SetMethod(http.MethodPost).
	SetCtrl("Users", "Create").
	Register()

// Edit: GET /users/:id/edit
var UserEdit = usersGroup.Route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Edit").
	RegisterWithID()

// Update: PUT /users/:id
var UserUpdate = usersGroup.Route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("Users", "Update").
	RegisterWithID()

// Destroy: DELETE /users/:id
var UserDestroy = usersGroup.Route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("Users", "Destroy").
	RegisterWithID()

package routes

import "net/http"

const (
	usersRoutePrefix = "/users"
	usersNamePrefix  = "users"
)

var usersGroup = newRouteGroup(usersNamePrefix, usersRoutePrefix)

var UserIndex = usersGroup.route("index").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Index").
	Register()

var UserShow = usersGroup.route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Show").
	RegisterWithID()

var UserNew = usersGroup.route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "New").
	Register()

var UserCreate = usersGroup.route("create").
	SetMethod(http.MethodPost).
	SetCtrl("Users", "Create").
	Register()

var UserEdit = usersGroup.route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Edit").
	RegisterWithID()

var UserUpdate = usersGroup.route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("Users", "Update").
	RegisterWithID()

var UserDestroy = usersGroup.route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("Users", "Destroy").
	RegisterWithID()

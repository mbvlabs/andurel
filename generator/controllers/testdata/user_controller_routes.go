package routes

import "net/http"

const (
	usersRoutePrefix = "/users"
	usersNamePrefix  = "users"
)

var usersGroup = NewRouteGroup(usersNamePrefix, usersRoutePrefix)

var UserIndex = usersGroup.Route("index").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Index").
	Register()

var UserShow = usersGroup.Route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Show").
	RegisterWithID()

var UserNew = usersGroup.Route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "New").
	Register()

var UserCreate = usersGroup.Route("create").
	SetMethod(http.MethodPost).
	SetCtrl("Users", "Create").
	Register()

var UserEdit = usersGroup.Route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("Users", "Edit").
	RegisterWithID()

var UserUpdate = usersGroup.Route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("Users", "Update").
	RegisterWithID()

var UserDestroy = usersGroup.Route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("Users", "Destroy").
	RegisterWithID()

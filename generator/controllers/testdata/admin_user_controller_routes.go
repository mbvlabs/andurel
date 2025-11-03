package routes

import "net/http"

const (
	adminUsersRoutePrefix = "/admin_users"
	adminUsersNamePrefix  = "admin_users"
)

var adminUsersGroup = NewRouteGroup(adminUsersNamePrefix, adminUsersRoutePrefix)

var AdminUserIndex = adminUsersGroup.Route("index").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Index").
	Register()

var AdminUserShow = adminUsersGroup.Route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Show").
	RegisterWithID()

var AdminUserNew = adminUsersGroup.Route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "New").
	Register()

var AdminUserCreate = adminUsersGroup.Route("create").
	SetMethod(http.MethodPost).
	SetCtrl("AdminUsers", "Create").
	Register()

var AdminUserEdit = adminUsersGroup.Route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Edit").
	RegisterWithID()

var AdminUserUpdate = adminUsersGroup.Route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("AdminUsers", "Update").
	RegisterWithID()

var AdminUserDestroy = adminUsersGroup.Route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("AdminUsers", "Destroy").
	RegisterWithID()

package routes

import "net/http"

const (
	adminUsersRoutePrefix = "/admin_users"
	adminUsersNamePrefix  = "admin_users"
)

var adminUsersGroup = newRouteGroup(adminUsersNamePrefix, adminUsersRoutePrefix)

var AdminUserIndex = adminUsersGroup.route("index").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Index").
	Register()

var AdminUserShow = adminUsersGroup.route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Show").
	RegisterWithID()

var AdminUserNew = adminUsersGroup.route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "New").
	Register()

var AdminUserCreate = adminUsersGroup.route("create").
	SetMethod(http.MethodPost).
	SetCtrl("AdminUsers", "Create").
	Register()

var AdminUserEdit = adminUsersGroup.route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Edit").
	RegisterWithID()

var AdminUserUpdate = adminUsersGroup.route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("AdminUsers", "Update").
	RegisterWithID()

var AdminUserDestroy = adminUsersGroup.route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("AdminUsers", "Destroy").
	RegisterWithID()

package routes

import "net/http"

const (
	adminUsersRoutePrefix = "/admin_users"
	adminUsersNamePrefix  = "admin_users"
)

var adminUsersGroup = newRouteGroup(adminUsersNamePrefix, adminUsersRoutePrefix)

var AdminUserIndex = adminUsersGroup.route("index").
	Register()

var AdminUserShow = adminUsersGroup.route("show").
	SetPath("/:id").
	RegisterWithID()

var AdminUserNew = adminUsersGroup.route("new").
	SetPath("/new").
	Register()

var AdminUserCreate = adminUsersGroup.route("create").
	Register()

var AdminUserEdit = adminUsersGroup.route("edit").
	SetPath("/:id/edit").
	RegisterWithID()

var AdminUserUpdate = adminUsersGroup.route("update").
	SetPath("/:id").
	RegisterWithID()

var AdminUserDestroy = adminUsersGroup.route("destroy").
	SetPath("/:id").
	RegisterWithID()

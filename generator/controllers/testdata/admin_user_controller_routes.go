package routes

import "net/http"

const (
	adminUsersRoutePrefix = "/admin_users"
	adminUsersNamePrefix  = "admin_users"
)

var adminUsersGroup = NewRouteGroup(adminUsersNamePrefix, adminUsersRoutePrefix)

// Index: GET /admin_users
var AdminUserIndex = adminUsersGroup.Route("index").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Index").
	Register()

// Show: GET /admin_users/:id
var AdminUserShow = adminUsersGroup.Route("show").
	SetPath("/:id").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Show").
	RegisterWithID()

// New: GET /admin_users/new
var AdminUserNew = adminUsersGroup.Route("new").
	SetPath("/new").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "New").
	Register()

// Create: POST /admin_users
var AdminUserCreate = adminUsersGroup.Route("create").
	SetMethod(http.MethodPost).
	SetCtrl("AdminUsers", "Create").
	Register()

// Edit: GET /admin_users/:id/edit
var AdminUserEdit = adminUsersGroup.Route("edit").
	SetPath("/:id/edit").
	SetMethod(http.MethodGet).
	SetCtrl("AdminUsers", "Edit").
	RegisterWithID()

// Update: PUT /admin_users/:id
var AdminUserUpdate = adminUsersGroup.Route("update").
	SetPath("/:id").
	SetMethod(http.MethodPut).
	SetCtrl("AdminUsers", "Update").
	RegisterWithID()

// Destroy: DELETE /admin_users/:id
var AdminUserDestroy = adminUsersGroup.Route("destroy").
	SetPath("/:id").
	SetMethod(http.MethodDelete).
	SetCtrl("AdminUsers", "Destroy").
	RegisterWithID()

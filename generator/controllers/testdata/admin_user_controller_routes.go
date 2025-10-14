package routes

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	adminUsersRoutePrefix = "/admin_users"
	adminUsersNamePrefix  = "admin_users"
)

var AdminUserRoutes = []Route{
	AdminUserIndex,
	AdminUserShow.Route,
	AdminUserNew,
	AdminUserCreate,
	AdminUserEdit.Route,
	AdminUserUpdate.Route,
	AdminUserDestroy.Route,
}

var AdminUserIndex = Route{
	Name:             adminUsersNamePrefix + ".index",
	Path:             adminUsersRoutePrefix,
	Method:           http.MethodGet,
	Controller:       "AdminUsers",
	ControllerMethod: "Index",
}

var AdminUserShow = adminUsersShow{
	Route: Route{
		Name:             adminUsersNamePrefix + ".show",
		Path:             adminUsersRoutePrefix + "/:id",
		Method:           http.MethodGet,
		Controller:       "AdminUsers",
		ControllerMethod: "Show",
	},
}

type adminUsersShow struct {
	Route
}

func (r adminUsersShow) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var AdminUserNew = Route{
	Name:             adminUsersNamePrefix + ".new",
	Path:             adminUsersRoutePrefix + "/new",
	Method:           http.MethodGet,
	Controller:       "AdminUsers",
	ControllerMethod: "New",
}

var AdminUserCreate = Route{
	Name:             adminUsersNamePrefix + ".create",
	Path:             adminUsersRoutePrefix,
	Method:           http.MethodPost,
	Controller:       "AdminUsers",
	ControllerMethod: "Create",
}

var AdminUserEdit = adminUsersEdit{
	Route: Route{
		Name:             adminUsersNamePrefix + ".edit",
		Path:             adminUsersRoutePrefix + "/:id/edit",
		Method:           http.MethodGet,
		Controller:       "AdminUsers",
		ControllerMethod: "Edit",
	},
}

type adminUsersEdit struct {
	Route
}

func (r adminUsersEdit) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var AdminUserUpdate = adminUsersUpdate{
	Route: Route{
		Name:             adminUsersNamePrefix + ".update",
		Path:             adminUsersRoutePrefix + "/:id",
		Method:           http.MethodPut,
		Controller:       "AdminUsers",
		ControllerMethod: "Update",
	},
}

type adminUsersUpdate struct {
	Route
}

func (r adminUsersUpdate) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var AdminUserDestroy = adminUsersDestroy{
	Route: Route{
		Name:             adminUsersNamePrefix + ".destroy",
		Path:             adminUsersRoutePrefix + "/:id",
		Method:           http.MethodDelete,
		Controller:       "AdminUsers",
		ControllerMethod: "Destroy",
	},
}

type adminUsersDestroy struct {
	Route
}

func (r adminUsersDestroy) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

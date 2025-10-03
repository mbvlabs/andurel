package routes

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	usersRoutePrefix = "/users"
	usersNamePrefix  = "users"
)

var UserRoutes = []Route{
	UserIndex,
	UserShow.Route,
	UserNew,
	UserCreate,
	UserEdit.Route,
	UserUpdate.Route,
	UserDestroy.Route,
}

var UserIndex = Route{
	Name:             usersNamePrefix + ".index",
	Path:             usersRoutePrefix,
	Method:           http.MethodGet,
	Controller:       "Users",
	ControllerMethod: "Index",
}

var UserShow = usersShow{
	Route: Route{
		Name:             usersNamePrefix + ".show",
		Path:             usersRoutePrefix + "/:id",
		Method:           http.MethodGet,
		Controller:       "Users",
		ControllerMethod: "Show",
	},
}

type usersShow struct {
	Route
}

func (r usersShow) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var UserNew = Route{
	Name:             usersNamePrefix + ".new",
	Path:             usersRoutePrefix + "/new",
	Method:           http.MethodGet,
	Controller:       "Users",
	ControllerMethod: "New",
}

var UserCreate = Route{
	Name:             usersNamePrefix + ".create",
	Path:             usersRoutePrefix,
	Method:           http.MethodPost,
	Controller:       "Users",
	ControllerMethod: "Create",
}

var UserEdit = usersEdit{
	Route: Route{
		Name:             usersNamePrefix + ".edit",
		Path:             usersRoutePrefix + "/:id/edit",
		Method:           http.MethodGet,
		Controller:       "Users",
		ControllerMethod: "Edit",
	},
}

type usersEdit struct {
	Route
}

func (r usersEdit) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var UserUpdate = usersUpdate{
	Route: Route{
		Name:             usersNamePrefix + ".update",
		Path:             usersRoutePrefix + "/:id",
		Method:           http.MethodPut,
		Controller:       "Users",
		ControllerMethod: "Update",
	},
}

type usersUpdate struct {
	Route
}

func (r usersUpdate) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

var UserDestroy = usersDestroy{
	Route: Route{
		Name:             usersNamePrefix + ".destroy",
		Path:             usersRoutePrefix + "/:id",
		Method:           http.MethodDelete,
		Controller:       "Users",
		ControllerMethod: "Destroy",
	},
}

type usersDestroy struct {
	Route
}

func (r usersDestroy) GetPath(id uuid.UUID) string {
	return strings.Replace(r.Path, ":id", id.String(), 1)
}

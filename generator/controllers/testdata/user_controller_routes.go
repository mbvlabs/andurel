package routes

const (
	usersRoutePrefix = "/users"
	usersNamePrefix  = "users"
)

var usersGroup = newRouteGroup(usersNamePrefix, usersRoutePrefix)

var UserIndex = usersGroup.route("index").
	Register()

var UserShow = usersGroup.route("show").
	SetPath("/:id").
	RegisterWithID()

var UserNew = usersGroup.route("new").
	SetPath("/new").
	Register()

var UserCreate = usersGroup.route("create").
	Register()

var UserEdit = usersGroup.route("edit").
	SetPath("/:id/edit").
	RegisterWithID()

var UserUpdate = usersGroup.route("update").
	SetPath("/:id").
	RegisterWithID()

var UserDestroy = usersGroup.route("destroy").
	SetPath("/:id").
	RegisterWithID()

package routes

const (
	productsRoutePrefix = "/products"
	productsNamePrefix  = "products"
)

var productsGroup = newRouteGroup(productsNamePrefix, productsRoutePrefix)

var ProductIndex = productsGroup.route("index").
	Register()

var ProductShow = productsGroup.route("show").
	SetPath("/:id").
	RegisterWithID()

var ProductNew = productsGroup.route("new").
	SetPath("/new").
	Register()

var ProductCreate = productsGroup.route("create").
	Register()

var ProductEdit = productsGroup.route("edit").
	SetPath("/:id/edit").
	RegisterWithID()

var ProductUpdate = productsGroup.route("update").
	SetPath("/:id").
	RegisterWithID()

var ProductDestroy = productsGroup.route("destroy").
	SetPath("/:id").
	RegisterWithID()

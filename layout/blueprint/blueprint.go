// Package blueprint provides structured types for scaffold configuration that
// support additive merges from multiple extensions without conflicts.
package blueprint

import "sort"

// Blueprint holds all structured configuration for a scaffold project. Each
// section supports additive operations that maintain uniqueness and ordering.
type Blueprint struct {
	// Tools lists go tool dependencies for the go.mod tool directive
	Tools *OrderedSet

	// Controllers section
	Controllers ControllerSection

	// Routes section
	Routes RouteSection

	// Models section
	Models ModelSection

	// Configuration section
	Config ConfigSection

	// Migrations and database
	Migrations MigrationSection

	// Main holds configuration for the main.go application setup
	Main MainSection

	// Cookies section for router/cookies package
	Cookies CookiesSection
}

// ControllerSection holds controller-related configuration.
type ControllerSection struct {
	// Import paths needed by controllers
	Imports *OrderedSet

	// Controller dependencies (parameters for New function)
	Dependencies []Dependency

	// Controller fields to add to the Controllers struct
	Fields []Field

	// Constructor initializations (e.g., "pages := newPages(db, cache)")
	Constructors []Constructor
}

// RouteSection holds routing configuration.
type RouteSection struct {
	// Route definitions
	Routes []Route

	// Route groups (e.g., "auth" for authRoutes, "admin" for adminRoutes)
	// Used to aggregate routes in the router_routes_routes.tmpl aggregator
	RouteGroups *OrderedSet

	// RouteCollections holds grouped route expressions to include in BuildRoutes.
	RouteCollections []RouteCollection

	// Route group imports (for middleware, etc.)
	Imports *OrderedSet

	// RouteRegistrations holds route registration entries for the registrar function
	Registrations []RouteRegistration

	// RegistrationFunctions holds grouped registration functions
	RegistrationFunctions []RegistrationFunction
}

// ModelSection holds model configuration.
type ModelSection struct {
	// Model imports
	Imports *OrderedSet

	// Model struct definitions
	Models []Model
}

// ConfigSection holds application configuration.
type ConfigSection struct {
	// Config struct fields
	Fields []Field

	// Environment variable mappings
	EnvVars []EnvVar
}

// MigrationSection holds database migration information.
type MigrationSection struct {
	// Migration file paths
	Migrations []Migration
}

// MainSection holds application startup configuration.
type MainSection struct {
	// Import paths needed in main.go (beyond controller dependencies)
	Imports *OrderedSet

	// Initialization code blocks (e.g., service creation)
	Initializations []Initialization

	// Background workers to start
	BackgroundWorkers []BackgroundWorker

	// Pre-run hooks executed before server starts
	PreRunHooks []PreRunHook
}

// Initialization represents a service/dependency initialization in main.go
type Initialization struct {
	// VarName is the variable name (e.g., "emailSender")
	VarName string
	// Expression is the initialization code (e.g., "email.NewMailpit()")
	Expression string
	// DependsOn lists variable names this depends on (for ordering)
	DependsOn []string
	// Order for deterministic rendering
	Order int
}

// BackgroundWorker represents a goroutine to start in main.go
type BackgroundWorker struct {
	// Name is a descriptive name for the worker
	Name string
	// FunctionCall is the function to call (e.g., "worker.StartQueue(ctx, queue)")
	FunctionCall string
	// DependsOn lists variables this worker needs
	DependsOn []string
	// Order for deterministic rendering
	Order int
}

// PreRunHook represents setup code to run before starting the server
type PreRunHook struct {
	// Name is a descriptive name for the hook
	Name string
	// Code is the Go code to execute (e.g., "if err := migrate(db); err != nil { return err }")
	Code string
	// Order for deterministic rendering
	Order int
}

// CookiesSection holds cookies package configuration
type CookiesSection struct {
	Imports           *OrderedSet
	Constants         []Constant
	AppFields         []Field
	Functions         []Function
	CreateSessionCode string
	GetSessionCode    string
}

// Constant represents a const declaration
type Constant struct {
	Name  string
	Value string
	Order int
}

// Function represents a function definition
type Function struct {
	Name  string
	Code  string
	Order int
}

// Dependency represents a constructor parameter.
type Dependency struct {
	Name string
	Type string
	// InitExpr is the optional initialization expression (e.g., "queue.NewInMemoryQueue()")
	// If empty, the dependency is assumed to be provided externally (like db)
	InitExpr string
	// ImportPath is the import path needed when InitExpr is provided
	// (e.g., "myapp/queue" for "queue.NewInMemoryQueue()")
	ImportPath string
	// Order for deterministic rendering
	Order int
}

// Field represents a struct field.
type Field struct {
	Name string
	Type string
	// Order for deterministic rendering
	Order int
}

// Constructor represents an initialization statement in a constructor.
type Constructor struct {
	// VarName is the variable name on the left side (e.g., "pages")
	VarName string
	// FieldName is the struct field this variable should be assigned to (e.g., "Pages")
	FieldName string
	// Type is the type of the controller (e.g., "*controllers.Pages")
	Type string
	// Expression is the right-hand side (e.g., "newPages(db, cache)")
	Expression string
	// Order for deterministic rendering
	Order int
}

// Route represents a route definition.
type Route struct {
	Name             string
	Path             string
	Controller       string
	ControllerMethod string
	Method           string
	IncludeInSitemap bool
	// Order for deterministic rendering
	Order int
}

// RouteCollection represents grouped route variables for BuildRoutes.
type RouteCollection struct {
	Routes []string
	// Order for deterministic rendering
	Order int
}

// RouteRegistration represents a route registration entry for the registrar function.
type RouteRegistration struct {
	// Method is the HTTP method (e.g., "http.MethodGet")
	Method string
	// RouteVariable is the route variable name (e.g., "routes.Health")
	RouteVariable string
	// HandlerRef is the handler method reference (e.g., "api.Health")
	HandlerRef string
	// Middleware is optional middleware to apply to this route
	Middleware []string
	// Order for deterministic rendering
	Order int
}

// RegistrationFunction represents a named registration function that groups route registrations.
type RegistrationFunction struct {
	// FunctionName is the name of the registration function (e.g., "registerAuthRoutes")
	FunctionName string
	// ControllerVarName is the variable name of the controller (e.g., "usersController")
	ControllerVarName string
	// Registrations contains all route registrations for this function
	Registrations []RouteRegistration
	// Order for deterministic rendering
	Order int
}

// Model represents a model struct definition.
type Model struct {
	Name   string
	Fields []Field
	// Order for deterministic rendering
	Order int
}

// EnvVar represents an environment variable mapping.
type EnvVar struct {
	Key          string
	ConfigField  string
	DefaultValue string
	// Order for deterministic rendering
	Order int
}

// Migration represents a database migration.
type Migration struct {
	Name      string
	Timestamp string
	Path      string
	// Order for deterministic rendering (usually by timestamp)
	Order int
}

// New creates a new Blueprint with initialized sections.
func New() *Blueprint {
	return &Blueprint{
		Tools: NewOrderedSet(),

		Controllers: ControllerSection{
			Imports:      NewOrderedSet(),
			Dependencies: make([]Dependency, 0),
			Fields:       make([]Field, 0),
			Constructors: make([]Constructor, 0),
		},
		Routes: RouteSection{
			Routes:                make([]Route, 0),
			RouteGroups:           NewOrderedSet(),
			RouteCollections:      make([]RouteCollection, 0),
			Imports:               NewOrderedSet(),
			Registrations:         make([]RouteRegistration, 0),
			RegistrationFunctions: make([]RegistrationFunction, 0),
		},
		Models: ModelSection{
			Imports: NewOrderedSet(),
			Models:  make([]Model, 0),
		},
		Config: ConfigSection{
			Fields:  make([]Field, 0),
			EnvVars: make([]EnvVar, 0),
		},
		Migrations: MigrationSection{
			Migrations: make([]Migration, 0),
		},
		Main: MainSection{
			Imports:           NewOrderedSet(),
			Initializations:   make([]Initialization, 0),
			BackgroundWorkers: make([]BackgroundWorker, 0),
			PreRunHooks:       make([]PreRunHook, 0),
		},
		Cookies: CookiesSection{
			Imports:   NewOrderedSet(),
			Constants: make([]Constant, 0),
			AppFields: make([]Field, 0),
			Functions: make([]Function, 0),
		},
	}
}

// SortedDependencies returns controller dependencies sorted by order.
func (cs *ControllerSection) SortedDependencies() []Dependency {
	deps := make([]Dependency, len(cs.Dependencies))
	copy(deps, cs.Dependencies)
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Order < deps[j].Order
	})
	return deps
}

// SortedFields returns controller fields sorted by order.
func (cs *ControllerSection) SortedFields() []Field {
	fields := make([]Field, len(cs.Fields))
	copy(fields, cs.Fields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Order < fields[j].Order
	})
	return fields
}

// SortedConstructors returns constructors sorted by order.
func (cs *ControllerSection) SortedConstructors() []Constructor {
	ctors := make([]Constructor, len(cs.Constructors))
	copy(ctors, cs.Constructors)
	sort.Slice(ctors, func(i, j int) bool {
		return ctors[i].Order < ctors[j].Order
	})
	return ctors
}

// SortedRoutes returns routes sorted by order.
func (rs *RouteSection) SortedRoutes() []Route {
	routes := make([]Route, len(rs.Routes))
	copy(routes, rs.Routes)
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Order < routes[j].Order
	})
	return routes
}

// SortedRouteCollections returns grouped route expressions sorted by order.
func (rs *RouteSection) SortedRouteCollections() []RouteCollection {
	if len(rs.RouteCollections) == 0 {
		return nil
	}

	result := make([]RouteCollection, len(rs.RouteCollections))
	copy(result, rs.RouteCollections)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Order < result[j].Order
	})
	return result
}

// SortedRegistrations returns route registrations sorted by order.
func (rs *RouteSection) SortedRegistrations() []RouteRegistration {
	registrations := make([]RouteRegistration, len(rs.Registrations))
	copy(registrations, rs.Registrations)
	sort.Slice(registrations, func(i, j int) bool {
		return registrations[i].Order < registrations[j].Order
	})
	return registrations
}

// SortedRegistrationFunctions returns registration functions sorted by order.
func (rs *RouteSection) SortedRegistrationFunctions() []RegistrationFunction {
	functions := make([]RegistrationFunction, len(rs.RegistrationFunctions))
	copy(functions, rs.RegistrationFunctions)
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Order < functions[j].Order
	})
	return functions
}

// SortedModels returns models sorted by order.
func (ms *ModelSection) SortedModels() []Model {
	models := make([]Model, len(ms.Models))
	copy(models, ms.Models)
	sort.Slice(models, func(i, j int) bool {
		return models[i].Order < models[j].Order
	})
	return models
}

// SortedFields returns config fields sorted by order.
func (cs *ConfigSection) SortedFields() []Field {
	fields := make([]Field, len(cs.Fields))
	copy(fields, cs.Fields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Order < fields[j].Order
	})
	return fields
}

// SortedEnvVars returns environment variables sorted by order.
func (cs *ConfigSection) SortedEnvVars() []EnvVar {
	envVars := make([]EnvVar, len(cs.EnvVars))
	copy(envVars, cs.EnvVars)
	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Order < envVars[j].Order
	})
	return envVars
}

// SortedMigrations returns migrations sorted by order.
func (ms *MigrationSection) SortedMigrations() []Migration {
	migrations := make([]Migration, len(ms.Migrations))
	copy(migrations, ms.Migrations)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Order < migrations[j].Order
	})
	return migrations
}

// SortedInitializations returns initializations sorted by order.
func (ms *MainSection) SortedInitializations() []Initialization {
	inits := make([]Initialization, len(ms.Initializations))
	copy(inits, ms.Initializations)
	sort.Slice(inits, func(i, j int) bool {
		return inits[i].Order < inits[j].Order
	})
	return inits
}

// SortedBackgroundWorkers returns background workers sorted by order.
func (ms *MainSection) SortedBackgroundWorkers() []BackgroundWorker {
	workers := make([]BackgroundWorker, len(ms.BackgroundWorkers))
	copy(workers, ms.BackgroundWorkers)
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].Order < workers[j].Order
	})
	return workers
}

// SortedPreRunHooks returns pre-run hooks sorted by order.
func (ms *MainSection) SortedPreRunHooks() []PreRunHook {
	hooks := make([]PreRunHook, len(ms.PreRunHooks))
	copy(hooks, ms.PreRunHooks)
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Order < hooks[j].Order
	})
	return hooks
}

// SortedConstants returns cookies constants sorted by order.
func (cs *CookiesSection) SortedConstants() []Constant {
	constants := make([]Constant, len(cs.Constants))
	copy(constants, cs.Constants)
	sort.Slice(constants, func(i, j int) bool {
		return constants[i].Order < constants[j].Order
	})
	return constants
}

// SortedAppFields returns cookies app fields sorted by order.
func (cs *CookiesSection) SortedAppFields() []Field {
	fields := make([]Field, len(cs.AppFields))
	copy(fields, cs.AppFields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Order < fields[j].Order
	})
	return fields
}

// SortedFunctions returns cookies functions sorted by order.
func (cs *CookiesSection) SortedFunctions() []Function {
	functions := make([]Function, len(cs.Functions))
	copy(functions, cs.Functions)
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Order < functions[j].Order
	})
	return functions
}

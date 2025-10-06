// Package blueprint provides structured types for scaffold configuration that
// support additive merges from multiple extensions without conflicts.
package blueprint

import "sort"

// Blueprint holds all structured configuration for a scaffold project. Each
// section supports additive operations that maintain uniqueness and ordering.
type Blueprint struct {
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

	// Route group imports (for middleware, etc.)
	Imports *OrderedSet
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
	// Expression is the right-hand side (e.g., "newPages(db, cache)")
	Expression string
	// Order for deterministic rendering
	Order int
}

// Route represents a route definition.
type Route struct {
	Name             string
	Path             string
	Handler          string
	HandleMethod     string
	Method           string
	IncludeInSitemap bool
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
		Controllers: ControllerSection{
			Imports:      NewOrderedSet(),
			Dependencies: make([]Dependency, 0),
			Fields:       make([]Field, 0),
			Constructors: make([]Constructor, 0),
		},
		Routes: RouteSection{
			Routes:      make([]Route, 0),
			RouteGroups: NewOrderedSet(),
			Imports:     NewOrderedSet(),
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

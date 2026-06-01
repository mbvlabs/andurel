package blueprint

import "sort"

type Blueprint struct {
	Tools       *OrderedSet
	Controllers ControllerSection
	Routes      RouteSection
	Models      ModelSection
	Config      ConfigSection
	Migrations  MigrationSection
	Main        MainSection
	Cookies     CookiesSection
}

type ControllerSection struct {
	Imports *OrderedSet
}

type RouteSection struct {
	Routes                []Route
	RouteGroups           *OrderedSet
	RouteCollections      []RouteCollection
	Imports               *OrderedSet
	Registrations         []RouteRegistration
	RegistrationFunctions []RegistrationFunction
}

type ModelSection struct {
	Imports *OrderedSet
	Models  []Model
}

type ConfigSection struct {
	Fields  []Field
	EnvVars []EnvVar
}

type MigrationSection struct {
	Migrations []Migration
}

type WorkerDependency struct {
	Name string
	Type string
}

type MainSection struct {
	Imports                *OrderedSet
	ServiceProvides        []string
	WorkerDependencies     []WorkerDependency
	ExtraControllerProvides []string
	PreRunHooks            []PreRunHook
}

type PreRunHook struct {
	Name  string
	Code  string
	Order int
}

type CookiesSection struct {
	Imports           *OrderedSet
	Constants         []Constant
	AppFields         []Field
	Functions         []Function
	CreateSessionCode string
	GetSessionCode    string
}

type Constant struct {
	Name  string
	Value string
	Order int
}

type Function struct {
	Name  string
	Code  string
	Order int
}

type Field struct {
	Name  string
	Type  string
	Order int
}

type Route struct {
	Name             string
	Path             string
	Controller       string
	ControllerMethod string
	Method           string
	IncludeInSitemap bool
	Order            int
}

type RouteCollection struct {
	Routes []string
	Order  int
}

type RouteRegistration struct {
	Method        string
	RouteVariable string
	HandlerRef    string
	Middleware    []string
	Order         int
}

type RegistrationFunction struct {
	FunctionName      string
	ControllerVarName string
	Registrations     []RouteRegistration
	Order             int
}

type Model struct {
	Name   string
	Fields []Field
	Order  int
}

type EnvVar struct {
	Key          string
	ConfigField  string
	DefaultValue string
	Order        int
}

type Migration struct {
	Name      string
	Timestamp string
	Path      string
	Order     int
}

func New() *Blueprint {
	return &Blueprint{
		Tools: NewOrderedSet(),

		Controllers: ControllerSection{
			Imports: NewOrderedSet(),
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
			Imports:                NewOrderedSet(),
			ServiceProvides:        make([]string, 0),
			WorkerDependencies:     make([]WorkerDependency, 0),
			ExtraControllerProvides: make([]string, 0),
			PreRunHooks:            make([]PreRunHook, 0),
		},
		Cookies: CookiesSection{
			Imports:   NewOrderedSet(),
			Constants: make([]Constant, 0),
			AppFields: make([]Field, 0),
			Functions: make([]Function, 0),
		},
	}
}

func (rs *RouteSection) SortedRoutes() []Route {
	routes := make([]Route, len(rs.Routes))
	copy(routes, rs.Routes)
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Order < routes[j].Order
	})
	return routes
}

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

func (rs *RouteSection) SortedRegistrations() []RouteRegistration {
	registrations := make([]RouteRegistration, len(rs.Registrations))
	copy(registrations, rs.Registrations)
	sort.Slice(registrations, func(i, j int) bool {
		return registrations[i].Order < registrations[j].Order
	})
	return registrations
}

func (rs *RouteSection) SortedRegistrationFunctions() []RegistrationFunction {
	functions := make([]RegistrationFunction, len(rs.RegistrationFunctions))
	copy(functions, rs.RegistrationFunctions)
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Order < functions[j].Order
	})
	return functions
}

func (ms *ModelSection) SortedModels() []Model {
	models := make([]Model, len(ms.Models))
	copy(models, ms.Models)
	sort.Slice(models, func(i, j int) bool {
		return models[i].Order < models[j].Order
	})
	return models
}

func (cs *ConfigSection) SortedFields() []Field {
	fields := make([]Field, len(cs.Fields))
	copy(fields, cs.Fields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Order < fields[j].Order
	})
	return fields
}

func (cs *ConfigSection) SortedEnvVars() []EnvVar {
	envVars := make([]EnvVar, len(cs.EnvVars))
	copy(envVars, cs.EnvVars)
	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Order < envVars[j].Order
	})
	return envVars
}

func (ms *MigrationSection) SortedMigrations() []Migration {
	migrations := make([]Migration, len(ms.Migrations))
	copy(migrations, ms.Migrations)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Order < migrations[j].Order
	})
	return migrations
}

func (ms *MainSection) SortedPreRunHooks() []PreRunHook {
	hooks := make([]PreRunHook, len(ms.PreRunHooks))
	copy(hooks, ms.PreRunHooks)
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Order < hooks[j].Order
	})
	return hooks
}

func (cs *CookiesSection) SortedConstants() []Constant {
	constants := make([]Constant, len(cs.Constants))
	copy(constants, cs.Constants)
	sort.Slice(constants, func(i, j int) bool {
		return constants[i].Order < constants[j].Order
	})
	return constants
}

func (cs *CookiesSection) SortedAppFields() []Field {
	fields := make([]Field, len(cs.AppFields))
	copy(fields, cs.AppFields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Order < fields[j].Order
	})
	return fields
}

func (cs *CookiesSection) SortedFunctions() []Function {
	functions := make([]Function, len(cs.Functions))
	copy(functions, cs.Functions)
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Order < functions[j].Order
	})
	return functions
}

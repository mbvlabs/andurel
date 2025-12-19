package blueprint

import (
	"fmt"
	"strings"
	"unicode"
)

// Builder provides a typed API for adding elements to a blueprint while
// enforcing uniqueness and maintaining deterministic ordering.
type Builder struct {
	bp *Blueprint
	// Track next order values for each category
	nextControllerDepOrder        int
	nextControllerFieldOrder      int
	nextConstructorOrder          int
	nextRouteOrder                int
	nextRouteCollectionOrder      int
	nextRouteRegistrationOrder    int
	nextRegistrationFunctionOrder int
	nextModelOrder                int
	nextConfigFieldOrder          int
	nextEnvVarOrder               int
	nextMigrationOrder            int
	nextInitializationOrder       int
	nextBackgroundWorkerOrder     int
	nextPreRunHookOrder           int
	nextCookiesConstantOrder      int
	nextCookiesAppFieldOrder      int
	nextCookiesFunctionOrder      int
	// Track current registration function being built
	currentRegistrationFunction *RegistrationFunction
}

// NewBuilder creates a builder wrapping the provided blueprint.
func NewBuilder(bp *Blueprint) *Builder {
	if bp == nil {
		bp = New()
	}

	return &Builder{
		bp: bp,
	}
}

// Blueprint returns the underlying blueprint.
func (b *Builder) Blueprint() *Blueprint {
	return b.bp
}

// AddTool registers a go tool binary to include in the go.mod tool directive.
func (b *Builder) AddTool(tool string) *Builder {
	if b == nil || b.bp == nil || tool == "" {
		return b
	}

	if b.bp.Tools == nil {
		b.bp.Tools = NewOrderedSet()
	}

	b.bp.Tools.Add(tool)
	return b
}

// AddControllerImport adds an import path to the controllers section.
func (b *Builder) AddControllerImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Controllers.Imports.Add(importPath)
	}
	return b
}

// AddControllerDependency adds a dependency parameter to the controller
// constructor. The order is automatically assigned based on insertion sequence.
// Use AddControllerDependencyWithInit to provide an initialization expression.
func (b *Builder) AddControllerDependency(name, typeName string) *Builder {
	return b.AddControllerDependencyWithInit(name, typeName, "")
}

// AddControllerDependencyWithInit adds a dependency with an optional initialization expression.
// If initExpr is provided, it will be used to initialize the dependency in main.go.
// If initExpr is empty, the dependency is assumed to be provided externally (like db).
func (b *Builder) AddControllerDependencyWithInit(name, typeName, initExpr string) *Builder {
	return b.addControllerDependencyWithInitAndImport(name, typeName, initExpr, "")
}

// addControllerDependencyWithInitAndImport adds a dependency with initialization expression
// and the import path needed for that expression. This is the internal implementation used
// by the extension API.
func (b *Builder) addControllerDependencyWithInitAndImport(
	name, typeName, initExpr, importPath string,
) *Builder {
	if name == "" || typeName == "" {
		return b
	}

	// Check if already exists by name
	for _, dep := range b.bp.Controllers.Dependencies {
		if dep.Name == name {
			return b // Already exists, skip
		}
	}

	b.bp.Controllers.Dependencies = append(b.bp.Controllers.Dependencies, Dependency{
		Name:       name,
		Type:       typeName,
		InitExpr:   initExpr,
		ImportPath: importPath,
		Order:      b.nextControllerDepOrder,
	})
	b.nextControllerDepOrder++
	return b
}

// AddControllerField adds a field to the Controllers struct.
func (b *Builder) AddControllerField(name, typeName string) *Builder {
	if name == "" || typeName == "" {
		return b
	}

	// Check if already exists by name
	for _, field := range b.bp.Controllers.Fields {
		if field.Name == name {
			return b
		}
	}

	b.bp.Controllers.Fields = append(b.bp.Controllers.Fields, Field{
		Name:  name,
		Type:  typeName,
		Order: b.nextControllerFieldOrder,
	})
	b.nextControllerFieldOrder++
	return b
}

// AddControllerConstructor adds a constructor initialization statement.
// The fieldName is automatically derived by finding a matching controller field.
// If no match is found, it capitalizes the first letter of varName.
// TODO: naming
func (b *Builder) AddControllerConstructor(varName, expression string) *Builder {
	if varName == "" || expression == "" {
		return b
	}

	// Check if already exists by varName
	for _, ctor := range b.bp.Controllers.Constructors {
		if ctor.VarName == varName {
			return b
		}
	}

	// Try to find matching field by case-insensitive comparison
	fieldName := capitalizeFirst(varName) // default
	varNameLower := strings.ToLower(varName)
	for _, field := range b.bp.Controllers.Fields {
		if strings.ToLower(field.Name) == varNameLower {
			fieldName = field.Name
			break
		}
	}

	b.bp.Controllers.Constructors = append(b.bp.Controllers.Constructors, Constructor{
		VarName:    varName,
		FieldName:  fieldName,
		Expression: expression,
		Order:      b.nextConstructorOrder,
	})
	b.nextConstructorOrder++
	return b
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// AddRoute adds a route definition.
func (b *Builder) AddRoute(route Route) *Builder {
	if route.Name == "" || route.Path == "" {
		return b
	}

	// Check if already exists by name
	for _, r := range b.bp.Routes.Routes {
		if r.Name == route.Name {
			return b
		}
	}

	route.Order = b.nextRouteOrder
	b.bp.Routes.Routes = append(b.bp.Routes.Routes, route)
	b.nextRouteOrder++
	return b
}

// AddRouteImport adds an import to the routes section.
func (b *Builder) AddRouteImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Routes.Imports.Add(importPath)
	}
	return b
}

// AddRouteGroup adds a route group name (e.g., "auth" for authRoutes).
// These are used by the router aggregator template to combine all route groups.
func (b *Builder) AddRouteGroup(groupName string) *Builder {
	if groupName != "" {
		b.bp.Routes.RouteGroups.Add(groupName)
	}
	return b
}

// AddRouteCollection records route expressions to include in the BuildRoutes literal.
func (b *Builder) AddRouteCollection(routes ...string) *Builder {
	if b == nil || b.bp == nil || len(routes) == 0 {
		return b
	}

	cleaned := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		if _, exists := seen[route]; exists {
			continue
		}
		seen[route] = struct{}{}
		cleaned = append(cleaned, route)
	}

	if len(cleaned) == 0 {
		return b
	}

	b.bp.Routes.RouteCollections = append(
		b.bp.Routes.RouteCollections,
		RouteCollection{
			Routes: cleaned,
			Order:  b.nextRouteCollectionOrder,
		},
	)
	b.nextRouteCollectionOrder++
	return b
}

// AddRouteRegistration adds a route registration entry for the registrar function.
// The method is the HTTP method constant (e.g., "http.MethodGet"),
// routeVariable is the route variable (e.g., "routes.Health"),
// controllerRef is the controller method reference (e.g., "ctrls.API.Health"),
// and middleware is optional middleware to apply.
// If a registration function is currently being built (via StartRouteRegistrationFunction),
// the registration is added to that function. Otherwise, it's added to the main registrations.
func (b *Builder) AddRouteRegistration(method, routeVariable, controllerRef string, middleware ...string) *Builder {
	if b == nil || b.bp == nil {
		return b
	}

	if method == "" || routeVariable == "" || controllerRef == "" {
		return b
	}

	registration := RouteRegistration{
		Method:        method,
		RouteVariable: routeVariable,
		ControllerRef: controllerRef,
		Middleware:    middleware,
		Order:         b.nextRouteRegistrationOrder,
	}
	b.nextRouteRegistrationOrder++

	if b.currentRegistrationFunction != nil {
		b.currentRegistrationFunction.Registrations = append(
			b.currentRegistrationFunction.Registrations,
			registration,
		)
	} else {
		b.bp.Routes.Registrations = append(b.bp.Routes.Registrations, registration)
	}

	return b
}

// StartRouteRegistrationFunction begins building a registration function.
// All subsequent AddRouteRegistration calls will be added to this function
// until EndRouteRegistrationFunction is called.
func (b *Builder) StartRouteRegistrationFunction(functionName string) *Builder {
	if b == nil || b.bp == nil {
		return b
	}

	if functionName == "" {
		return b
	}

	if b.currentRegistrationFunction != nil {
		return b
	}

	b.currentRegistrationFunction = &RegistrationFunction{
		FunctionName:  functionName,
		Registrations: make([]RouteRegistration, 0),
		Order:         b.nextRegistrationFunctionOrder,
	}
	b.nextRegistrationFunctionOrder++

	return b
}

// EndRouteRegistrationFunction completes the current registration function
// and adds it to the blueprint.
func (b *Builder) EndRouteRegistrationFunction() *Builder {
	if b == nil || b.bp == nil {
		return b
	}

	if b.currentRegistrationFunction == nil {
		return b
	}

	for _, existing := range b.bp.Routes.RegistrationFunctions {
		if existing.FunctionName == b.currentRegistrationFunction.FunctionName {
			b.currentRegistrationFunction = nil
			return b
		}
	}

	b.bp.Routes.RegistrationFunctions = append(
		b.bp.Routes.RegistrationFunctions,
		*b.currentRegistrationFunction,
	)
	b.currentRegistrationFunction = nil

	return b
}

// AddModel adds a model definition.
func (b *Builder) AddModel(model Model) *Builder {
	if model.Name == "" {
		return b
	}

	// Check if already exists by name
	for _, m := range b.bp.Models.Models {
		if m.Name == model.Name {
			return b
		}
	}

	model.Order = b.nextModelOrder
	b.bp.Models.Models = append(b.bp.Models.Models, model)
	b.nextModelOrder++
	return b
}

// AddModelImport adds an import to the models section.
func (b *Builder) AddModelImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Models.Imports.Add(importPath)
	}
	return b
}

// AddConfigField adds a field to the config struct.
func (b *Builder) AddConfigField(name, typeName string) *Builder {
	if name == "" || typeName == "" {
		return b
	}

	// Check if already exists by name
	for _, field := range b.bp.Config.Fields {
		if field.Name == name {
			return b
		}
	}

	b.bp.Config.Fields = append(b.bp.Config.Fields, Field{
		Name:  name,
		Type:  typeName,
		Order: b.nextConfigFieldOrder,
	})
	b.nextConfigFieldOrder++
	return b
}

// AddEnvVar adds an environment variable mapping.
func (b *Builder) AddEnvVar(key, configField, defaultValue string) *Builder {
	if key == "" || configField == "" {
		return b
	}

	// Check if already exists by key
	for _, ev := range b.bp.Config.EnvVars {
		if ev.Key == key {
			return b
		}
	}

	b.bp.Config.EnvVars = append(b.bp.Config.EnvVars, EnvVar{
		Key:          key,
		ConfigField:  configField,
		DefaultValue: defaultValue,
		Order:        b.nextEnvVarOrder,
	})
	b.nextEnvVarOrder++
	return b
}

// AddMigration adds a migration definition.
func (b *Builder) AddMigration(migration Migration) *Builder {
	if migration.Name == "" {
		return b
	}

	// Check if already exists by name
	for _, m := range b.bp.Migrations.Migrations {
		if m.Name == migration.Name {
			return b
		}
	}

	migration.Order = b.nextMigrationOrder
	b.bp.Migrations.Migrations = append(b.bp.Migrations.Migrations, migration)
	b.nextMigrationOrder++
	return b
}

// AddMainImport adds an import path to the main.go file.
func (b *Builder) AddMainImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Main.Imports.Add(importPath)
	}
	return b
}

// AddMainInitialization adds an initialization code block to main.go.
// The varName is the variable name, expression is the initialization code.
// DependsOn can be used to specify ordering dependencies.
func (b *Builder) AddMainInitialization(varName, expression string, dependsOn ...string) *Builder {
	if varName == "" || expression == "" {
		return b
	}

	// Check if already exists by varName
	for _, init := range b.bp.Main.Initializations {
		if init.VarName == varName {
			return b
		}
	}

	b.bp.Main.Initializations = append(b.bp.Main.Initializations, Initialization{
		VarName:    varName,
		Expression: expression,
		DependsOn:  dependsOn,
		Order:      b.nextInitializationOrder,
	})
	b.nextInitializationOrder++
	return b
}

// AddBackgroundWorker adds a background worker goroutine to main.go.
func (b *Builder) AddBackgroundWorker(name, functionCall string, dependsOn ...string) *Builder {
	if name == "" || functionCall == "" {
		return b
	}

	// Check if already exists by name
	for _, worker := range b.bp.Main.BackgroundWorkers {
		if worker.Name == name {
			return b
		}
	}

	b.bp.Main.BackgroundWorkers = append(b.bp.Main.BackgroundWorkers, BackgroundWorker{
		Name:         name,
		FunctionCall: functionCall,
		DependsOn:    dependsOn,
		Order:        b.nextBackgroundWorkerOrder,
	})
	b.nextBackgroundWorkerOrder++
	return b
}

// AddPreRunHook adds a pre-run hook to execute before the server starts.
func (b *Builder) AddPreRunHook(name, code string) *Builder {
	if name == "" || code == "" {
		return b
	}

	// Check if already exists by name
	for _, hook := range b.bp.Main.PreRunHooks {
		if hook.Name == name {
			return b
		}
	}

	b.bp.Main.PreRunHooks = append(b.bp.Main.PreRunHooks, PreRunHook{
		Name:  name,
		Code:  code,
		Order: b.nextPreRunHookOrder,
	})
	b.nextPreRunHookOrder++
	return b
}

// AddCookiesImport adds an import path to the cookies section.
func (b *Builder) AddCookiesImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Cookies.Imports.Add(importPath)
	}
	return b
}

// AddCookiesConstant adds a constant to the cookies section.
func (b *Builder) AddCookiesConstant(name, value string) *Builder {
	if name == "" || value == "" {
		return b
	}

	for _, c := range b.bp.Cookies.Constants {
		if c.Name == name {
			return b
		}
	}

	b.bp.Cookies.Constants = append(b.bp.Cookies.Constants, Constant{
		Name:  name,
		Value: value,
		Order: b.nextCookiesConstantOrder,
	})
	b.nextCookiesConstantOrder++
	return b
}

// AddCookiesAppField adds a field to the App struct in cookies.
func (b *Builder) AddCookiesAppField(name, typeName string) *Builder {
	if name == "" || typeName == "" {
		return b
	}

	for _, f := range b.bp.Cookies.AppFields {
		if f.Name == name {
			return b
		}
	}

	b.bp.Cookies.AppFields = append(b.bp.Cookies.AppFields, Field{
		Name:  name,
		Type:  typeName,
		Order: b.nextCookiesAppFieldOrder,
	})
	b.nextCookiesAppFieldOrder++
	return b
}

// AddCookiesFunction adds a function to the cookies section.
func (b *Builder) AddCookiesFunction(name, code string) *Builder {
	if name == "" || code == "" {
		return b
	}

	for _, f := range b.bp.Cookies.Functions {
		if f.Name == name {
			return b
		}
	}

	b.bp.Cookies.Functions = append(b.bp.Cookies.Functions, Function{
		Name:  name,
		Code:  code,
		Order: b.nextCookiesFunctionOrder,
	})
	b.nextCookiesFunctionOrder++
	return b
}

// SetCookiesCreateSessionCode sets the code to execute when creating a session.
func (b *Builder) SetCookiesCreateSessionCode(code string) *Builder {
	b.bp.Cookies.CreateSessionCode = code
	return b
}

// SetCookiesGetSessionCode sets the code to execute when getting session data.
func (b *Builder) SetCookiesGetSessionCode(code string) *Builder {
	b.bp.Cookies.GetSessionCode = code
	return b
}

// Merge combines another blueprint into this one, maintaining uniqueness and
// order. Items from the other blueprint are added after existing items.
func (b *Builder) Merge(other *Blueprint) error {
	if other == nil {
		return fmt.Errorf("blueprint: cannot merge nil blueprint")
	}

	// Merge controller imports
	b.bp.Controllers.Imports.Merge(other.Controllers.Imports)

	// Merge go tools
	if b.bp.Tools == nil {
		b.bp.Tools = NewOrderedSet()
	}
	b.bp.Tools.Merge(other.Tools)

	// Merge controller dependencies (check for duplicates by name)
	for _, dep := range other.Controllers.Dependencies {
		b.addControllerDependencyWithInitAndImport(dep.Name, dep.Type, dep.InitExpr, dep.ImportPath)
	}

	// Merge controller fields
	for _, field := range other.Controllers.Fields {
		b.AddControllerField(field.Name, field.Type)
	}

	// Merge constructors
	for _, ctor := range other.Controllers.Constructors {
		b.AddControllerConstructor(ctor.VarName, ctor.Expression)
	}

	// Merge routes
	b.bp.Routes.Imports.Merge(other.Routes.Imports)
	b.bp.Routes.RouteGroups.Merge(other.Routes.RouteGroups)
	for _, route := range other.Routes.Routes {
		b.AddRoute(route)
	}
	for _, collection := range other.Routes.RouteCollections {
		b.AddRouteCollection(collection.Routes...)
	}
	for _, registration := range other.Routes.Registrations {
		b.AddRouteRegistration(registration.Method, registration.RouteVariable, registration.ControllerRef, registration.Middleware...)
	}
	for _, regFunc := range other.Routes.RegistrationFunctions {
		b.StartRouteRegistrationFunction(regFunc.FunctionName)
		for _, registration := range regFunc.Registrations {
			b.AddRouteRegistration(registration.Method, registration.RouteVariable, registration.ControllerRef, registration.Middleware...)
		}
		b.EndRouteRegistrationFunction()
	}

	// Merge models
	b.bp.Models.Imports.Merge(other.Models.Imports)
	for _, model := range other.Models.Models {
		b.AddModel(model)
	}

	// Merge config
	for _, field := range other.Config.Fields {
		b.AddConfigField(field.Name, field.Type)
	}
	for _, envVar := range other.Config.EnvVars {
		b.AddEnvVar(envVar.Key, envVar.ConfigField, envVar.DefaultValue)
	}

	// Merge migrations
	for _, migration := range other.Migrations.Migrations {
		b.AddMigration(migration)
	}

	// Merge main section
	b.bp.Main.Imports.Merge(other.Main.Imports)
	for _, init := range other.Main.Initializations {
		b.AddMainInitialization(init.VarName, init.Expression, init.DependsOn...)
	}
	for _, worker := range other.Main.BackgroundWorkers {
		b.AddBackgroundWorker(worker.Name, worker.FunctionCall, worker.DependsOn...)
	}
	for _, hook := range other.Main.PreRunHooks {
		b.AddPreRunHook(hook.Name, hook.Code)
	}

	// Merge cookies section
	b.bp.Cookies.Imports.Merge(other.Cookies.Imports)
	for _, c := range other.Cookies.Constants {
		b.AddCookiesConstant(c.Name, c.Value)
	}
	for _, f := range other.Cookies.AppFields {
		b.AddCookiesAppField(f.Name, f.Type)
	}
	for _, f := range other.Cookies.Functions {
		b.AddCookiesFunction(f.Name, f.Code)
	}
	if other.Cookies.CreateSessionCode != "" {
		b.SetCookiesCreateSessionCode(other.Cookies.CreateSessionCode)
	}
	if other.Cookies.GetSessionCode != "" {
		b.SetCookiesGetSessionCode(other.Cookies.GetSessionCode)
	}

	return nil
}

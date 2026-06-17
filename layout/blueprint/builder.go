package blueprint

import (
	"fmt"
	"strings"
	"unicode"
)

type Builder struct {
	bp *Blueprint
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
	nextPreRunHookOrder           int
	nextCookiesConstantOrder      int
	nextCookiesAppFieldOrder      int
	nextCookiesFunctionOrder      int
	currentRegistrationFunction *RegistrationFunction
}

func NewBuilder(bp *Blueprint) *Builder {
	if bp == nil {
		bp = New()
	}
	return &Builder{
		bp: bp,
	}
}

func (b *Builder) Blueprint() *Blueprint {
	return b.bp
}

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

func (b *Builder) AddControllerImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Controllers.Imports.Add(importPath)
	}
	return b
}

func (b *Builder) AddRoute(route Route) *Builder {
	if route.Name == "" || route.Path == "" {
		return b
	}
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

func (b *Builder) AddRouteImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Routes.Imports.Add(importPath)
	}
	return b
}

func (b *Builder) AddRouteGroup(groupName string) *Builder {
	if groupName != "" {
		b.bp.Routes.RouteGroups.Add(groupName)
	}
	return b
}

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
		HandlerRef:    controllerRef,
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

func (b *Builder) StartRouteRegistrationFunction(functionName string, controllerVarName string) *Builder {
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
		FunctionName:      functionName,
		ControllerVarName: controllerVarName,
		Registrations:     make([]RouteRegistration, 0),
		Order:             b.nextRegistrationFunctionOrder,
	}
	b.nextRegistrationFunctionOrder++
	return b
}

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

func (b *Builder) AddModel(model Model) *Builder {
	if model.Name == "" {
		return b
	}
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

func (b *Builder) AddModelImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Models.Imports.Add(importPath)
	}
	return b
}

func (b *Builder) AddConfigField(name, typeName string) *Builder {
	if name == "" || typeName == "" {
		return b
	}
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

func (b *Builder) AddEnvVar(key, configField, defaultValue string) *Builder {
	if key == "" || configField == "" {
		return b
	}
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

func (b *Builder) AddMigration(migration Migration) *Builder {
	if migration.Name == "" {
		return b
	}
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

func (b *Builder) AddMainImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Main.Imports.Add(importPath)
	}
	return b
}

func (b *Builder) AddServiceProvide(expr string) *Builder {
	if expr == "" {
		return b
	}
	for _, s := range b.bp.Main.ServiceProvides {
		if s == expr {
			return b
		}
	}
	b.bp.Main.ServiceProvides = append(b.bp.Main.ServiceProvides, expr)
	return b
}

func (b *Builder) AddWorkerDependency(name, typeName string) *Builder {
	if name == "" || typeName == "" {
		return b
	}
	for i, d := range b.bp.Main.WorkerDependencies {
		if d.Name == name {
			b.bp.Main.WorkerDependencies[i].Type = typeName
			return b
		}
	}
	b.bp.Main.WorkerDependencies = append(b.bp.Main.WorkerDependencies, WorkerDependency{Name: name, Type: typeName})
	return b
}

func (b *Builder) AddPreRunHook(name, code string) *Builder {
	if name == "" || code == "" {
		return b
	}
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

func (b *Builder) AddCookiesImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Cookies.Imports.Add(importPath)
	}
	return b
}

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

func (b *Builder) SetCookiesCreateSessionCode(code string) *Builder {
	b.bp.Cookies.CreateSessionCode = code
	return b
}

func (b *Builder) SetCookiesGetSessionCode(code string) *Builder {
	b.bp.Cookies.GetSessionCode = code
	return b
}

func (b *Builder) Merge(other *Blueprint) error {
	if other == nil {
		return fmt.Errorf("blueprint: cannot merge nil blueprint")
	}

	b.bp.Controllers.Imports.Merge(other.Controllers.Imports)

	if b.bp.Tools == nil {
		b.bp.Tools = NewOrderedSet()
	}
	b.bp.Tools.Merge(other.Tools)

	b.bp.Routes.Imports.Merge(other.Routes.Imports)
	b.bp.Routes.RouteGroups.Merge(other.Routes.RouteGroups)
	for _, route := range other.Routes.Routes {
		b.AddRoute(route)
	}
	for _, collection := range other.Routes.RouteCollections {
		b.AddRouteCollection(collection.Routes...)
	}
	for _, registration := range other.Routes.Registrations {
		b.AddRouteRegistration(registration.Method, registration.RouteVariable, registration.HandlerRef, registration.Middleware...)
	}
	for _, regFunc := range other.Routes.RegistrationFunctions {
		b.StartRouteRegistrationFunction(regFunc.FunctionName, regFunc.ControllerVarName)
		for _, registration := range regFunc.Registrations {
			b.AddRouteRegistration(registration.Method, registration.RouteVariable, registration.HandlerRef, registration.Middleware...)
		}
		b.EndRouteRegistrationFunction()
	}

	b.bp.Models.Imports.Merge(other.Models.Imports)
	for _, model := range other.Models.Models {
		b.AddModel(model)
	}

	for _, field := range other.Config.Fields {
		b.AddConfigField(field.Name, field.Type)
	}
	for _, envVar := range other.Config.EnvVars {
		b.AddEnvVar(envVar.Key, envVar.ConfigField, envVar.DefaultValue)
	}

	for _, migration := range other.Migrations.Migrations {
		b.AddMigration(migration)
	}

	b.bp.Main.Imports.Merge(other.Main.Imports)
	for _, s := range other.Main.ServiceProvides {
		b.AddServiceProvide(s)
	}
	for _, dep := range other.Main.WorkerDependencies {
		b.AddWorkerDependency(dep.Name, dep.Type)
	}
	for _, hook := range other.Main.PreRunHooks {
		b.AddPreRunHook(hook.Name, hook.Code)
	}

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

func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

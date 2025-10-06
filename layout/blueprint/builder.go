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
	nextControllerDepOrder   int
	nextControllerFieldOrder int
	nextConstructorOrder     int
	nextRouteOrder           int
	nextModelOrder           int
	nextConfigFieldOrder     int
	nextEnvVarOrder          int
	nextMigrationOrder       int
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

// AddCtrlImport adds an import path to the controllers section.
func (b *Builder) AddCtrlImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Controllers.Imports.Add(importPath)
	}
	return b
}

// AddCtrlDependency adds a dependency parameter to the controller
// constructor. The order is automatically assigned based on insertion sequence.
// Use AddControllerDependencyWithInit to provide an initialization expression.
func (b *Builder) AddCtrlDependency(name, typeName string) *Builder {
	return b.AddCtrlDependencyWithInit(name, typeName, "")
}

// AddCtrlDependencyWithInit adds a dependency with an optional initialization expression.
// If initExpr is provided, it will be used to initialize the dependency in main.go.
// If initExpr is empty, the dependency is assumed to be provided externally (like db).
func (b *Builder) AddCtrlDependencyWithInit(name, typeName, initExpr string) *Builder {
	return b.addCtrlDependencyWithInitAndImport(name, typeName, initExpr, "")
}

// AddControllerDependencyWithInitAndImport adds a dependency with initialization expression
// and the import path needed for that expression. This is the internal implementation used
// by the extension API.
func (b *Builder) addCtrlDependencyWithInitAndImport(
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

// AddConstructor adds a constructor initialization statement.
// The fieldName is automatically derived by finding a matching controller field.
// If no match is found, it capitalizes the first letter of varName.
func (b *Builder) AddConstructor(varName, expression string) *Builder {
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

// Merge combines another blueprint into this one, maintaining uniqueness and
// order. Items from the other blueprint are added after existing items.
func (b *Builder) Merge(other *Blueprint) error {
	if other == nil {
		return fmt.Errorf("blueprint: cannot merge nil blueprint")
	}

	// Merge controller imports
	b.bp.Controllers.Imports.Merge(other.Controllers.Imports)

	// Merge controller dependencies (check for duplicates by name)
	for _, dep := range other.Controllers.Dependencies {
		b.addCtrlDependencyWithInitAndImport(dep.Name, dep.Type, dep.InitExpr, dep.ImportPath)
	}

	// Merge controller fields
	for _, field := range other.Controllers.Fields {
		b.AddControllerField(field.Name, field.Type)
	}

	// Merge constructors
	for _, ctor := range other.Controllers.Constructors {
		b.AddConstructor(ctor.VarName, ctor.Expression)
	}

	// Merge routes
	b.bp.Routes.Imports.Merge(other.Routes.Imports)
	b.bp.Routes.RouteGroups.Merge(other.Routes.RouteGroups)
	for _, route := range other.Routes.Routes {
		b.AddRoute(route)
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

	return nil
}

// BuilderAdapter wraps a *Builder to implement interfaces that require
// void return types. This adapter discards return values from the fluent
// builder methods.
type BuilderAdapter struct {
	*Builder
}

// NewBuilderAdapter creates an adapter wrapping the provided builder.
func NewBuilderAdapter(b *Builder) *BuilderAdapter {
	return &BuilderAdapter{Builder: b}
}

// The following methods implement interface contracts that require void returns.
// They delegate to the underlying Builder methods and discard return values.

func (a *BuilderAdapter) AddImport(importPath string) {
	a.Builder.AddCtrlImport(importPath)
}

func (a *BuilderAdapter) AddControllerDependency(name, typeName string) {
	a.Builder.AddCtrlDependency(name, typeName)
}

func (a *BuilderAdapter) AddControllerDependencyWithInit(name, typeName, initExpr string) {
	a.Builder.AddCtrlDependencyWithInit(name, typeName, initExpr)
}

func (a *BuilderAdapter) AddControllerDependencyWithInitAndImport(
	name, typeName, initExpr, importPath string,
) {
	a.addCtrlDependencyWithInitAndImport(name, typeName, initExpr, importPath)
}

func (a *BuilderAdapter) AddControllerField(name, typeName string) {
	a.Builder.AddControllerField(name, typeName)
}

func (a *BuilderAdapter) AddConstructor(varName, expression string) {
	a.Builder.AddConstructor(varName, expression)
}

func (a *BuilderAdapter) AddRouteImport(importPath string) {
	a.Builder.AddRouteImport(importPath)
}

func (a *BuilderAdapter) AddRouteGroup(groupName string) {
	a.Builder.AddRouteGroup(groupName)
}

func (a *BuilderAdapter) AddModelImport(importPath string) {
	a.Builder.AddModelImport(importPath)
}

func (a *BuilderAdapter) AddConfigField(name, typeName string) {
	a.Builder.AddConfigField(name, typeName)
}

func (a *BuilderAdapter) AddEnvVar(key, configField, defaultValue string) {
	a.Builder.AddEnvVar(key, configField, defaultValue)
}

// Ensure BuilderAdapter implements the extensions.Builder interface at compile time.
// This will fail to compile if the adapter doesn't properly implement the interface.
// The import is intentionally not added to avoid circular dependencies - this is just
// a compile-time check that will be validated when the adapter is used.
//
// Uncomment to verify (requires extensions import):
// var _ extensions.Builder = (*BuilderAdapter)(nil)

package blueprint

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mbvlabs/andurel/layout/extensions"
)

// Builder provides a typed API for adding elements to a blueprint while
// enforcing uniqueness and maintaining deterministic ordering.
type Builder struct {
	bp *Blueprint

	nextControllerDepOrder   int
	nextControllerFieldOrder int
	nextConstructorOrder     int
	nextRouteOrder           int
	nextModelOrder           int
	nextConfigFieldOrder     int
	nextEnvVarOrder          int
	nextMigrationOrder       int
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

func (b *Builder) addControllerImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Controllers.Imports.Add(importPath)
	}

	return b
}

func (b *Builder) AddControllerImport(importPath string) {
	b.addControllerImport(importPath)
}

func (b *Builder) AddControllerDependency(name, typeName string) {
	b.addControllerDependencyWithInitAndImport(name, typeName, "", "")
}

// AddControllerDependencyWithInit adds a dependency with an optional initialization expression.
// If initExpr is provided, it will be used to initialize the dependency in main.go.
// If initExpr is empty, the dependency is assumed to be provided externally (like db).
func (b *Builder) AddControllerDependencyWithInit(name, typeName, initExpr string) {
	b.addControllerDependencyWithInitAndImport(name, typeName, initExpr, "")
}

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

// AddControllerDependencyWithInitAndImport adds a dependency with initialization expression
// and the import path needed for that expression. This is the internal implementation used
// by the extension API.
func (b *Builder) AddControllerDependencyWithInitAndImport(
	name, typeName, initExpr, importPath string,
) {
	b.addControllerDependencyWithInitAndImport(name, typeName, initExpr, importPath)
}

func (b *Builder) addControllerField(name, typeName string) *Builder {
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

// AddControllerField adds a field to the Controllers struct.
func (b *Builder) AddControllerField(name, typeName string) {
	b.addControllerField(name, typeName)
}

// AddConstructor adds a constructor initialization statement.
// The fieldName is automatically derived by finding a matching controller field.
// If no match is found, it capitalizes the first letter of varName.
func (b *Builder) AddConstructor(varName, expression string) {
	b.addConstructor(varName, expression)
}

func (b *Builder) addConstructor(varName, expression string) *Builder {
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

func (b *Builder) addRouteImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Routes.Imports.Add(importPath)
	}
	return b
}

// AddRouteImport adds an import to the routes section.
func (b *Builder) AddRouteImport(importPath string) {
	b.addRouteImport(importPath)
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

func (b *Builder) addModelImport(importPath string) *Builder {
	if importPath != "" {
		b.bp.Models.Imports.Add(importPath)
	}
	return b
}

// AddModelImport adds an import to the models section.
func (b *Builder) AddModelImport(importPath string) {
	b.addModelImport(importPath)
}

func (b *Builder) addConfigField(name, typeName string) *Builder {
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

// AddConfigField adds a field to the config struct.
func (b *Builder) AddConfigField(name, typeName string) {
	b.addConfigField(name, typeName)
}

func (b *Builder) addEnvVar(key, configField, defaultValue string) *Builder {
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

// AddEnvVar adds an environment variable mapping.
func (b *Builder) AddEnvVar(key, configField, defaultValue string) {
	b.addEnvVar(key, configField, defaultValue)
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
		b.addControllerDependencyWithInitAndImport(dep.Name, dep.Type, dep.InitExpr, dep.ImportPath)
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

var _ extensions.Builder = (*Builder)(nil)

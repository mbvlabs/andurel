package controllers

import (
	"fmt"
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/templates"
	"mbvlabs/andurel/generator/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jinzhu/inflection"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ControllerType int

const (
	ResourceController ControllerType = iota
	NormalController
)

type GeneratedField struct {
	Name          string
	GoType        string
	GoFormType    string
	DBName        string
	IsSystemField bool
}

type GeneratedController struct {
	ResourceName string
	PluralName   string
	Package      string
	Fields       []GeneratedField
	ModulePath   string
	Type         ControllerType
}

type Config struct {
	ResourceName   string
	PluralName     string
	PackageName    string
	ModulePath     string
	ControllerType ControllerType
}

type Generator struct {
	typeMapper *types.TypeMapper
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper: types.NewTypeMapper(databaseType),
	}
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedController, error) {
	controller := &GeneratedController{
		ResourceName: config.ResourceName,
		PluralName:   config.PluralName,
		Package:      config.PackageName,
		ModulePath:   config.ModulePath,
		Type:         config.ControllerType,
		Fields:       make([]GeneratedField, 0),
	}

	// For resource controllers, we need to generate fields from the table
	if config.ControllerType == ResourceController {
		table, err := cat.GetTable("", config.PluralName)
		if err != nil {
			return nil, fmt.Errorf("table %s not found: %w", config.PluralName, err)
		}

		for _, col := range table.Columns {
			field, err := g.buildField(col)
			if err != nil {
				return nil, fmt.Errorf("failed to build field for column %s: %w", col.Name, err)
			}
			controller.Fields = append(controller.Fields, field)
		}
	}

	return controller, nil
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	goType, _, _, err := g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		return GeneratedField{}, err
	}

	field := GeneratedField{
		Name:          types.FormatFieldName(col.Name),
		GoType:        goType,
		DBName:        col.Name,
		IsSystemField: col.Name == "created_at" || col.Name == "updated_at" || col.Name == "id",
	}

	// Set form type for controller form handling
	switch goType {
	case "time.Time":
		field.GoFormType = "time.Time"
	case "int16":
		field.GoFormType = "int16"
	case "int32":
		field.GoFormType = "int32"
	case "int64":
		field.GoFormType = "int64"
	case "float32":
		field.GoFormType = "float32"
	case "float64":
		field.GoFormType = "float64"
	case "bool":
		field.GoFormType = "bool"
	default:
		field.GoFormType = "string"
	}

	return field, nil
}

func (g *Generator) RenderControllerFile(controller *GeneratedController) (string, error) {
	var templateName string
	switch controller.Type {
	case ResourceController:
		templateName = "resource_controller.tmpl"
	default:
		templateName = "controller.tmpl"
	}

	templateContent, err := templates.Files.ReadFile(templateName)
	if err != nil {
		return "", fmt.Errorf("failed to read controller template: %w", err)
	}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	tmpl, err := template.New("controller").Funcs(funcMap).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, controller); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) GenerateController(
	cat *catalog.Catalog,
	resourceName string,
	controllerType ControllerType,
	modulePath string,
) error {
	pluralName := inflection.Plural(strings.ToLower(resourceName))
	controllerPath := filepath.Join("controllers", pluralName+".go")

	// Check if controller already exists
	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	controller, err := g.Build(cat, Config{
		ResourceName:   resourceName,
		PluralName:     pluralName,
		PackageName:    "controllers",
		ModulePath:     modulePath,
		ControllerType: controllerType,
	})
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	controllerContent, err := g.RenderControllerFile(controller)
	if err != nil {
		return fmt.Errorf("failed to render controller file: %w", err)
	}

	// Ensure controllers directory exists
	if err := os.MkdirAll("controllers", 0755); err != nil {
		return fmt.Errorf("failed to create controllers directory: %w", err)
	}

	if err := os.WriteFile(controllerPath, []byte(controllerContent), 0600); err != nil {
		return fmt.Errorf("failed to write controller file: %w", err)
	}

	// Format the generated controller file
	if err := g.formatGoFile(controllerPath); err != nil {
		return fmt.Errorf("failed to format controller file: %w", err)
	}

	// For resource controllers, also generate routes
	if controllerType == ResourceController {
		if err := g.GenerateRoutes(resourceName, pluralName); err != nil {
			return fmt.Errorf("failed to generate routes: %w", err)
		}
	}

	return nil
}

func (g *Generator) GenerateRoutes(resourceName, pluralName string) error {
	routesPath := filepath.Join("router/routes", pluralName+".go")

	// Check if routes already exist
	if _, err := os.Stat(routesPath); err == nil {
		return fmt.Errorf("routes file %s already exists", routesPath)
	}

	routeContent, err := g.generateRouteContent(resourceName, pluralName)
	if err != nil {
		return fmt.Errorf("failed to generate route content: %w", err)
	}

	// Ensure routes directory exists
	if err := os.MkdirAll("router/routes", 0755); err != nil {
		return fmt.Errorf("failed to create routes directory: %w", err)
	}

	if err := os.WriteFile(routesPath, []byte(routeContent), 0600); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}

	// Format the generated routes file
	if err := g.formatGoFile(routesPath); err != nil {
		return fmt.Errorf("failed to format routes file: %w", err)
	}

	// Register routes in routes.go
	return g.registerRoutes(pluralName)
}

func (g *Generator) generateRouteContent(resourceName, pluralName string) (string, error) {
	templateContent, err := templates.Files.ReadFile("route.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read route template: %w", err)
	}

	data := struct {
		ResourceName string
		PluralName   string
	}{
		ResourceName: resourceName,
		PluralName:   pluralName,
	}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	tmpl, err := template.New("routes").Funcs(funcMap).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse routes template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute routes template: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) registerRoutes(pluralName string) error {
	routesFilePath := "router/routes/routes.go"

	content, err := os.ReadFile(routesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read routes.go: %w", err)
	}

	contentStr := string(content)

	// Add import for the new routes
	resourceName := cases.Title(language.English).String(strings.TrimSuffix(pluralName, "s")) // Convert plural to singular title case
	routeSliceName := resourceName + "Routes"

	// Check if routes are already registered
	if strings.Contains(contentStr, routeSliceName) {
		return nil // Already registered
	}

	// Find the return statement and insert before it
	lines := strings.Split(contentStr, "\n")
	var modifiedLines []string
	added := false

	for _, line := range lines {
		// Insert new routes before the return statement
		if strings.TrimSpace(line) == "return r" && !added {
			// Add the new routes
			modifiedLines = append(modifiedLines, "")
			modifiedLines = append(modifiedLines, fmt.Sprintf("\tr = append("))
			modifiedLines = append(modifiedLines, fmt.Sprintf("\t\tr,"))
			modifiedLines = append(modifiedLines, fmt.Sprintf("\t\t%s...,", routeSliceName))
			modifiedLines = append(modifiedLines, fmt.Sprintf("\t)"))
			modifiedLines = append(modifiedLines, "")
			added = true
		}
		modifiedLines = append(modifiedLines, line)
	}

	if !added {
		return fmt.Errorf("could not find appropriate place to register routes")
	}

	return os.WriteFile(routesFilePath, []byte(strings.Join(modifiedLines, "\n")), 0600)
}

func (g *Generator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}
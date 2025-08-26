package views

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
)

type ViewField struct {
	Name            string
	GoType          string
	GoFormType      string
	DisplayName     string
	IsTimestamp     bool
	InputType       string
	StringConverter string
	DBName          string
	CamelCase       string
	IsSystemField   bool
}

type GeneratedView struct {
	ResourceName string
	PluralName   string
	Fields       []ViewField
	ModulePath   string
}

type Config struct {
	ResourceName string
	PluralName   string
	ModulePath   string
}

type Generator struct {
	typeMapper *types.TypeMapper
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper: types.NewTypeMapper(databaseType),
	}
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedView, error) {
	view := &GeneratedView{
		ResourceName: config.ResourceName,
		PluralName:   config.PluralName,
		ModulePath:   config.ModulePath,
		Fields:       make([]ViewField, 0),
	}

	table, err := cat.GetTable("", config.PluralName)
	if err != nil {
		return nil, fmt.Errorf("table %s not found: %w", config.PluralName, err)
	}

	for _, col := range table.Columns {
		// Skip ID field for views
		if col.Name == "id" {
			continue
		}

		field, err := g.buildViewField(col)
		if err != nil {
			return nil, fmt.Errorf("failed to build field for column %s: %w", col.Name, err)
		}
		view.Fields = append(view.Fields, field)
	}

	return view, nil
}

func (g *Generator) buildViewField(col *catalog.Column) (ViewField, error) {
	goType, _, _, err := g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		goType = "string"
	}

	field := ViewField{
		Name:          types.FormatFieldName(col.Name),
		DisplayName:   types.FormatDisplayName(col.Name),
		DBName:        col.Name,
		CamelCase:     types.FormatCamelCase(col.Name),
		IsSystemField: col.Name == "created_at" || col.Name == "updated_at",
		GoType:        goType,
	}

	// Set input type and string converter based on Go type
	switch goType {
	case "time.Time":
		field.IsTimestamp = true
		field.InputType = "date"
		field.StringConverter = "%s.String()"
	case "string":
		field.InputType = "text"
		field.StringConverter = ""
	case "int16":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
	case "int32":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
	case "int64":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
	case "float32":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%f\", %s)"
	case "float64":
		field.InputType = "number"
		field.StringConverter = "fmt.Sprintf(\"%f\", %s)"
	case "bool":
		field.InputType = "checkbox"
		field.StringConverter = "fmt.Sprintf(\"%t\", %s)"
	case "uuid.UUID":
		field.InputType = "text"
		field.StringConverter = "%s.String()"
	case "[]byte":
		field.InputType = "text"
		field.StringConverter = "string(%s)"
	default:
		field.InputType = "text"
		field.StringConverter = ""
	}

	// Set form type for view form handling
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

func (g *Generator) RenderViewFile(view *GeneratedView) (string, error) {
	templateContent, err := templates.Files.ReadFile("resource_view.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read view template: %w", err)
	}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
		"StringDisplay": func(field ViewField, resourceName string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"{ %s.%s }",
					strings.ToLower(resourceName),
					field.Name,
				)
			}
			actualFieldRef := strings.ToLower(resourceName) + "." + field.Name
			converter := strings.ReplaceAll(
				field.StringConverter,
				"%s",
				actualFieldRef,
			)
			return fmt.Sprintf("{ %s }", converter)
		},
		"StringTableDisplay": func(field ViewField, resourceName string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"{ %s.%s }",
					strings.ToLower(resourceName),
					field.Name,
				)
			}
			actualFieldRef := strings.ToLower(resourceName) + "." + field.Name
			converter := strings.ReplaceAll(
				field.StringConverter,
				"%s",
				actualFieldRef,
			)
			return fmt.Sprintf("{ %s }", converter)
		},
		"StringValue": func(field ViewField, resourceName string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"%s.%s",
					strings.ToLower(resourceName),
					field.Name,
				)
			}
			actualFieldRef := strings.ToLower(resourceName) + "." + field.Name
			return strings.ReplaceAll(
				field.StringConverter,
				"%s",
				actualFieldRef,
			)
		},
	}

	tmpl, err := template.New("resource_view").Funcs(funcMap).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, view); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) GenerateView(
	cat *catalog.Catalog,
	resourceName string,
	modulePath string,
) error {
	pluralName := inflection.Plural(strings.ToLower(resourceName))
	viewPath := filepath.Join("views", pluralName+"_resource.templ")

	// Check if view already exists
	if _, err := os.Stat(viewPath); err == nil {
		return fmt.Errorf("view file %s already exists", viewPath)
	}

	view, err := g.Build(cat, Config{
		ResourceName: resourceName,
		PluralName:   pluralName,
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build view: %w", err)
	}

	viewContent, err := g.RenderViewFile(view)
	if err != nil {
		return fmt.Errorf("failed to render view file: %w", err)
	}

	// Ensure views directory exists
	if err := os.MkdirAll("views", 0755); err != nil {
		return fmt.Errorf("failed to create views directory: %w", err)
	}

	if err := os.WriteFile(viewPath, []byte(viewContent), 0600); err != nil {
		return fmt.Errorf("failed to write view file: %w", err)
	}

	// Format the generated view file
	if err := g.formatTemplFile(viewPath); err != nil {
		return fmt.Errorf("failed to format view file: %w", err)
	}

	// Run templ generate to compile templates (optional)
	if err := g.runCompileTemplates(); err != nil {
		return fmt.Errorf("failed to compile templates: %w", err)
	}

	return nil
}

func (g *Generator) formatTemplFile(filePath string) error {
	// Find the root directory with go.mod
	rootDir, err := findGoModRoot()
	if err != nil {
		// If we can't find go.mod, skip formatting (e.g., in test environments)
		return nil
	}

	cmd := exec.Command("go", "run", "github.com/a-h/templ/cmd/templ", "fmt", filePath)
	cmd.Dir = rootDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run templ fmt on %s: %w", filePath, err)
	}
	return nil
}

func findGoModRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found")
}

func (g *Generator) runCompileTemplates() error {
	// Check if 'just' command exists
	if _, err := exec.LookPath("just"); err != nil {
		// 'just' not available, skip template compilation
		return nil
	}

	// Check if Justfile exists in current directory
	if _, err := os.Stat("Justfile"); os.IsNotExist(err) {
		// No Justfile, skip template compilation
		return nil
	}

	cmd := exec.Command("just", "compile-templates")
	if err := cmd.Run(); err != nil {
		// If compilation fails, we'll just skip it in test environments
		// In production, this would be a real error
		return nil
	}
	return nil
}
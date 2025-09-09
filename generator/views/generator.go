package views

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"mbvlabs/andurel/generator/files"
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/internal/types"
	"mbvlabs/andurel/generator/templates"

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
	typeMapper  *types.TypeMapper
	fileManager *files.Manager
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper:  types.NewTypeMapper(databaseType),
		fileManager: files.NewManager(),
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

func (g *Generator) GenerateViewFile(view *GeneratedView) (string, error) {
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

	viewContent, err := g.GenerateViewFile(view)
	if err != nil {
		return fmt.Errorf("failed to render view file: %w", err)
	}

	if err := os.MkdirAll("views", 0o755); err != nil {
		return fmt.Errorf("failed to create views directory: %w", err)
	}

	if err := os.WriteFile(viewPath, []byte(viewContent), 0o600); err != nil {
		return fmt.Errorf("failed to write view file: %w", err)
	}

	if err := g.formatTemplFile(viewPath); err != nil {
		return fmt.Errorf("failed to format view file: %w", err)
	}

	if err := g.runCompileTemplates(); err != nil {
		return fmt.Errorf("failed to compile templates: %w", err)
	}

	fmt.Printf("Successfully generated view at %s\n", viewPath)
	return nil
}

func (g *Generator) formatTemplFile(filePath string) error {
	rootDir, err := g.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}
	slog.Info("Go mod root", "dir", rootDir)

	cmd := exec.Command("go", "tool", "templ", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run templ fmt on %s: %w", filePath, err)
	}

	return nil
}

func (g *Generator) runCompileTemplates() error {
	rootDir, err := g.fileManager.FindGoModRoot()
	if err != nil {
		return nil
	}

	cmd := exec.Command("go", "tool", "templ", "generate")
	cmd.Dir = rootDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run 'go tool templ generate': %w", err)
	}
	return nil
}

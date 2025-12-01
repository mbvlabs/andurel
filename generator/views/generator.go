package views

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
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
	TableName    string
	ModulePath   string
}

type Generator struct {
	typeMapper  *types.TypeMapper
	fileManager files.Manager
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper:  types.NewTypeMapper(databaseType),
		fileManager: files.NewUnifiedFileManager(),
	}
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedView, error) {
	view := &GeneratedView{
		ResourceName: config.ResourceName,
		PluralName:   config.PluralName,
		ModulePath:   config.ModulePath,
		Fields:       make([]ViewField, 0),
	}

	tableName := config.TableName
	if tableName == "" {
		tableName = config.PluralName
	}
	table, err := cat.GetTable("", tableName)
	if err != nil {
		return nil, errors.NewDatabaseError("get table", tableName, err)
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

func (g *Generator) GenerateViewFile(view *GeneratedView, withController bool) (string, error) {
	// Custom template functions for view-specific operations
	customFuncs := template.FuncMap{
		"StringDisplay": func(field ViewField, resourceName string) string {
			if field.StringConverter == "" {
				return fmt.Sprintf(
					"{ %s.%s }",
					strings.ToLower(resourceName),
					field.Name,
				)
			}
			var fieldRef strings.Builder
			fieldRef.Grow(len(resourceName) + len(field.Name) + 1)
			fieldRef.WriteString(strings.ToLower(resourceName))
			fieldRef.WriteString(".")
			fieldRef.WriteString(field.Name)
			actualFieldRef := fieldRef.String()
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
			var fieldRef strings.Builder
			fieldRef.Grow(len(resourceName) + len(field.Name) + 1)
			fieldRef.WriteString(strings.ToLower(resourceName))
			fieldRef.WriteString(".")
			fieldRef.WriteString(field.Name)
			actualFieldRef := fieldRef.String()
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
			var fieldRef strings.Builder
			fieldRef.Grow(len(resourceName) + len(field.Name) + 1)
			fieldRef.WriteString(strings.ToLower(resourceName))
			fieldRef.WriteString(".")
			fieldRef.WriteString(field.Name)
			actualFieldRef := fieldRef.String()
			return strings.ReplaceAll(
				field.StringConverter,
				"%s",
				actualFieldRef,
			)
		},
	}

	templateName := "resource_view_no_controller.tmpl"
	if withController {
		templateName = "resource_view.tmpl"
	}

	// Use the unified template service with custom functions
	service := templates.GetGlobalTemplateService()
	result, err := service.RenderTemplateWithCustomFunctions(templateName, view, customFuncs)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render view", templateName)
	}
	return result, nil
}

func (g *Generator) GenerateView(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	modulePath string,
) error {
	return g.GenerateViewWithController(cat, resourceName, tableName, modulePath, false)
}

func (g *Generator) GenerateViewWithController(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	modulePath string,
	withController bool,
) error {
	pluralName := naming.DeriveTableName(resourceName)
	viewPath := filepath.Join("views", tableName+"_resource.templ")

	if _, err := os.Stat(viewPath); err == nil {
		return fmt.Errorf("view file %s already exists", viewPath)
	}

	view, err := g.Build(cat, Config{
		ResourceName: resourceName,
		PluralName:   pluralName,
		TableName:    tableName,
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build view: %w", err)
	}

	viewContent, err := g.GenerateViewFile(view, withController)
	if err != nil {
		return fmt.Errorf("failed to render view file: %w", err)
	}

	if err := g.fileManager.EnsureDir("views"); err != nil {
		return err
	}

	if err := os.WriteFile(viewPath, []byte(viewContent), constants.FilePermissionPrivate); err != nil {
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
	var cmd *exec.Cmd

	if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
		cmd = exec.Command("go", "run", "github.com/a-h/templ/cmd/templ@"+versions.Templ, "fmt", filePath)
	} else {
		rootDir, err := g.fileManager.FindGoModRoot()
		if err != nil {
			return fmt.Errorf("failed to find project root: %w", err)
		}
		templBin := filepath.Join(rootDir, "bin", "templ")
		cmd = exec.Command(templBin, "fmt", filePath)
	}

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

	var cmd *exec.Cmd

	if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
		cmd = exec.Command("go", "run", "github.com/a-h/templ/cmd/templ@"+versions.Templ, "generate")
	} else {
		templBin := filepath.Join(rootDir, "bin", "templ")
		cmd = exec.Command(templBin, "generate")
	}

	cmd.Dir = rootDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run templ generate: %w", err)
	}
	return nil
}

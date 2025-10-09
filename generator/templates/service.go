package templates

import (
	"maps"
	"strings"
	"text/template"
	"unicode"

	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// TemplateData represents the unified data structure for all templates
type TemplateData struct {
	Resource ResourceData   `json:"resource"`
	Database DatabaseData   `json:"database"`
	Project  ProjectData    `json:"project"`
	Custom   map[string]any `json:"custom"`
}

// ResourceData contains resource-specific template data
type ResourceData struct {
	Name         string `json:"name"`
	PluralName   string `json:"plural_name"`
	Fields       any    `json:"fields"`
	ModulePath   string `json:"module_path"`
	Type         string `json:"type"`
	DatabaseType string `json:"database_type"`
}

// DatabaseData contains database-specific template data
type DatabaseData struct {
	Type   string `json:"type"`
	Method string `json:"method"`
	Driver string `json:"driver"`
}

// ProjectData contains project-specific template data
type ProjectData struct {
	ModulePath string `json:"module_path"`
	Name       string `json:"name"`
}

// TemplateService provides unified template rendering functionality
type TemplateService struct {
	cache     *TemplateCache
	functions template.FuncMap
}

// NewTemplateService creates a new template service with default functions
func NewTemplateService() *TemplateService {
	return &TemplateService{
		cache:     NewTemplateCache(),
		functions: getDefaultTemplateFunctions(),
	}
}

// TemplateBuilder provides a fluent interface for building template data
type TemplateBuilder struct {
	service *TemplateService
	data    *TemplateData
}

// NewTemplateBuilder creates a new template builder
func NewTemplateBuilder(service *TemplateService) *TemplateBuilder {
	return &TemplateBuilder{
		service: service,
		data: &TemplateData{
			Custom: make(map[string]any),
		},
	}
}

// WithResource sets resource data
func (tb *TemplateBuilder) WithResource(
	name, pluralName, modulePath, resourceType, databaseType string,
	fields any,
) *TemplateBuilder {
	tb.data.Resource = ResourceData{
		Name:         name,
		PluralName:   pluralName,
		Fields:       fields,
		ModulePath:   modulePath,
		Type:         resourceType,
		DatabaseType: databaseType,
	}
	return tb
}

// WithDatabase sets database data
func (tb *TemplateBuilder) WithDatabase(dbType, method, driver string) *TemplateBuilder {
	tb.data.Database = DatabaseData{
		Type:   dbType,
		Method: method,
		Driver: driver,
	}
	return tb
}

// WithProject sets project data
func (tb *TemplateBuilder) WithProject(modulePath, name string) *TemplateBuilder {
	tb.data.Project = ProjectData{
		ModulePath: modulePath,
		Name:       name,
	}
	return tb
}

// WithCustom adds custom data
func (tb *TemplateBuilder) WithCustom(key string, value any) *TemplateBuilder {
	tb.data.Custom[key] = value
	return tb
}

// Render renders the template with the built data
func (tb *TemplateBuilder) Render(templateName string) (string, error) {
	return tb.service.RenderTemplate(templateName, tb.data)
}

// RenderTemplate renders a template with the given data
func (ts *TemplateService) RenderTemplate(templateName string, data any) (string, error) {
	tmpl, err := ts.cache.GetTemplate(templateName, ts.functions)
	if err != nil {
		return "", errors.WrapTemplateError(err, "get template", templateName)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.WrapTemplateError(err, "execute template", templateName)
	}

	return buf.String(), nil
}

// RenderTemplateWithCustomFunctions renders a template with custom function map
func (ts *TemplateService) RenderTemplateWithCustomFunctions(
	templateName string,
	data any,
	funcMap template.FuncMap,
) (string, error) {
	// Merge default functions with custom functions
	mergedFuncs := make(template.FuncMap)
	maps.Copy(mergedFuncs, ts.functions)
	maps.Copy(mergedFuncs, funcMap)

	tmpl, err := ts.cache.GetTemplate(templateName, mergedFuncs)
	if err != nil {
		return "", errors.WrapTemplateError(err, "get template", templateName)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.WrapTemplateError(err, "execute template", templateName)
	}

	return buf.String(), nil
}

// getDefaultTemplateFunctions returns the default template function map
func getDefaultTemplateFunctions() template.FuncMap {
	return template.FuncMap{
		"ToLower":          strings.ToLower,
		"ToUpper":          strings.ToUpper,
		"ToSnakeCase":      naming.ToSnakeCase,
		"ToCamelCase":      naming.ToCamelCase,
		"ToLowerCamelCase": naming.ToLowerCamelCase,
		"DeriveTableName":  naming.DeriveTableName,
		"DatabaseType": func(data any) string {
			if td, ok := data.(*TemplateData); ok {
				return td.Database.Type
			}
			return ""
		},
		"DatabaseMethod": func(data any) string {
			if td, ok := data.(*TemplateData); ok {
				if td.Database.Method != "" {
					return td.Database.Method
				}
				return "Conn"
			}
			return "Conn"
		},
		"uuidParam": func(param string, databaseType string) string {
			if databaseType == "sqlite" {
				return param + ".String()"
			}
			return param
		},
		"toLowerCamelCase": toLowerCamelCase,
		"toCamelCase":      toCamelCase,
	}
}

// toCamelCase converts snake_case to camelCase for use in templates
func toCamelCase(s string) string {
	if s == "" {
		return ""
	}

	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s))

	// First part stays lowercase
	builder.WriteString(strings.ToLower(parts[0]))

	// Capitalize first letter of remaining parts
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			builder.WriteString(strings.ToUpper(parts[i][:1]))
			if len(parts[i]) > 1 {
				builder.WriteString(strings.ToLower(parts[i][1:]))
			}
		}
	}

	return builder.String()
}

// toLowerCamelCase converts PascalCase to camelCase for use in templates
func toLowerCamelCase(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}

	// Convert first character to lowercase
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// Global template service instance
var globalTemplateService = NewTemplateService()

// GetGlobalTemplateService returns the global template service instance
func GetGlobalTemplateService() *TemplateService {
	return globalTemplateService
}

// RenderTemplateUsingGlobal renders a template using the global service
func RenderTemplateUsingGlobal(templateName string, data any) (string, error) {
	return globalTemplateService.RenderTemplate(templateName, data)
}

// NewTemplateBuilderUsingGlobal creates a new template builder using the global service
func NewTemplateBuilderUsingGlobal() *TemplateBuilder {
	return NewTemplateBuilder(globalTemplateService)
}

package controllers

import (
	"strings"
	"text/template"
	"unicode"

	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/errors"
)

type TemplateRenderer struct{}

func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{}
}

func (tr *TemplateRenderer) RenderControllerFile(controller *GeneratedController) (string, error) {
	var templateName string
	switch controller.Type {
	case ResourceController:
		templateName = "resource_controller.tmpl"
	case ResourceControllerNoViews:
		templateName = "resource_controller_no_views.tmpl"
	default:
		templateName = "controller.tmpl"
	}

	funcMap := template.FuncMap{
		"ToLower":           strings.ToLower,
		"ToLowerCamelCase": tr.toLowerCamelCase,
		"ToCamelCase":       tr.toCamelCase,
		"DatabaseType": func() string {
			return controller.DatabaseType
		},
		"DatabaseMethod": func() string {
			return "Conn"
		},
		"uuidParam": func(param string) string {
			if controller.DatabaseType == "sqlite" {
				return param + ".String()"
			}
			return param
		},
	}

	tmpl, err := templates.GetCachedTemplate(templateName, funcMap)
	if err != nil {
		return "", errors.NewTemplateError(templateName, "get template", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, controller); err != nil {
		return "", errors.NewTemplateError(templateName, "execute template", err)
	}

	return buf.String(), nil
}

func (tr *TemplateRenderer) generateRouteContent(resourceName, pluralName string) (string, error) {
	data := struct {
		ResourceName string
		PluralName   string
	}{
		ResourceName: resourceName,
		PluralName:   pluralName,
	}

	funcMap := template.FuncMap{
		"ToLower":      strings.ToLower,
		"ToCamelCase": tr.toCamelCase,
	}

	tmpl, err := templates.GetCachedTemplate("route.tmpl", funcMap)
	if err != nil {
		return "", errors.NewTemplateError("route.tmpl", "get template", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.NewTemplateError("route.tmpl", "execute template", err)
	}

	return buf.String(), nil
}

// toCamelCase converts snake_case to camelCase for use in templates
func (tr *TemplateRenderer) toCamelCase(s string) string {
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
func (tr *TemplateRenderer) toLowerCamelCase(s string) string {
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

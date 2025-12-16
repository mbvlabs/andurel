package controllers

import (
	"text/template"

	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type TemplateRenderer struct {
	service *templates.TemplateService
}

func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		service: templates.GetGlobalTemplateService(),
	}
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

	// Custom template functions for controller-specific operations
	customFuncs := template.FuncMap{
		"DatabaseType": func() string {
			return controller.DatabaseType
		},
		"DatabaseMethod": func() string {
			return "Conn"
		},
		"uuidParam": func(param string) string {
			return param
		},
	}

	// Use the unified template service with custom functions and original data structure
	result, err := tr.service.RenderTemplateWithCustomFunctions(
		templateName,
		controller,
		customFuncs,
	)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render controller", templateName)
	}
	return result, nil
}

func (tr *TemplateRenderer) generateRouteContent(resourceName, pluralName string) (string, error) {
	// Create custom data structure for route template (router/routes/users.go)
	data := struct {
		ResourceName string
		PluralName   string
	}{
		ResourceName: resourceName,
		PluralName:   pluralName,
	}

	result, err := tr.service.RenderTemplate("route.tmpl", data)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render route", "route.tmpl")
	}
	return result, nil
}

func (tr *TemplateRenderer) generateRouteRegistrationFunction(resourceName, pluralName string) (string, error) {
	// Create custom data structure for route registration template
	data := struct {
		ResourceName          string
		PluralName            string
		CapitalizedPluralName string
	}{
		ResourceName:          resourceName,
		PluralName:            pluralName,
		CapitalizedPluralName: naming.Capitalize(naming.ToCamelCase(pluralName)),
	}

	result, err := tr.service.RenderTemplate("route_registration.tmpl", data)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render route registration", "route_registration.tmpl")
	}
	return result, nil
}

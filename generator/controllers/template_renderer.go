package controllers

import (
	"strings"
	"text/template"

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
		"ToLower": strings.ToLower,
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
		"ToLower": strings.ToLower,
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

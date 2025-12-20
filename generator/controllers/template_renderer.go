package controllers

import (
	"fmt"
	"os"
	"regexp"
	"strings"
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
	// Get module path
	modulePath, err := tr.getModulePath()
	if err != nil {
		return "", fmt.Errorf("failed to get module path: %w", err)
	}

	// Create custom data structure for route template (router/routes/users.go)
	data := struct {
		ResourceName string
		PluralName   string
		ModulePath   string
	}{
		ResourceName: resourceName,
		PluralName:   pluralName,
		ModulePath:   modulePath,
	}

	result, err := tr.service.RenderTemplate("route.tmpl", data)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render route", "route.tmpl")
	}
	return result, nil
}

func (tr *TemplateRenderer) generateRouteRegistrationFunction(resourceName, pluralName string) (string, error) {
	capitalizedPluralName := naming.Capitalize(naming.ToCamelCase(pluralName))
	lowercasePluralName := naming.ToLowerCamelCase(pluralName)

	// Extract controller parameters from router/router.go
	controllerParams, err := tr.extractControllerParams()
	if err != nil {
		return "", errors.WrapTemplateError(err, "extract controller params", "router/router.go")
	}

	// Add the new controller to the params
	controllerParamsWithNew := controllerParams + ", " + lowercasePluralName + " controllers." + capitalizedPluralName

	// Create custom data structure for route registration template
	data := struct {
		ResourceName          string
		PluralName            string
		CapitalizedPluralName string
		LowercasePluralName   string
		ControllerParams      string
	}{
		ResourceName:          resourceName,
		PluralName:            pluralName,
		CapitalizedPluralName: capitalizedPluralName,
		LowercasePluralName:   lowercasePluralName,
		ControllerParams:      controllerParamsWithNew,
	}

	result, err := tr.service.RenderTemplate("route_registration.tmpl", data)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render route registration", "route_registration.tmpl")
	}
	return result, nil
}

// extractControllerParams reads router/router.go and extracts the controller parameters
// from the RegisterCtrlRoutes method signature
func (tr *TemplateRenderer) extractControllerParams() (string, error) {
	routerPath := "router/router.go"
	content, err := os.ReadFile(routerPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", routerPath, err)
	}

	// Look for RegisterCtrlRoutes method signature
	// Pattern: func (r *Router) RegisterCtrlRoutes(\n\tmw middleware.Middleware,\n\t...\n)
	pattern := `func\s+\([^)]+\)\s+RegisterCtrlRoutes\s*\([^)]*mw\s+middleware\.Middleware([^)]*)\)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(content))

	if len(matches) < 2 {
		// If we can't find the method, return empty string (no existing controllers)
		return "", nil
	}

	// Extract the controller parameters (everything after mw middleware.Middleware)
	params := strings.TrimSpace(matches[1])
	if params == "" {
		return "", nil
	}

	// Remove leading comma if present
	params = strings.TrimPrefix(params, ",")
	params = strings.TrimSpace(params)

	return params, nil
}

// getModulePath reads go.mod to get the module path
func (tr *TemplateRenderer) getModulePath() (string, error) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

package controllers

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type TemplateRenderer struct {
	service *templates.TemplateService
}

type customRouteAction struct {
	Name       string
	MethodName string
	RouteName  string
	Path       string
}

func customRouteActions(actions []string) []customRouteAction {
	customActions := make([]customRouteAction, 0, len(actions))
	seen := map[string]struct{}{}
	for _, action := range actions {
		normalized := strings.ToLower(action)
		if slices.Contains(crudActions, normalized) {
			continue
		}
		routeName := naming.ToSnakeCase(action)
		if _, ok := seen[routeName]; ok {
			continue
		}
		seen[routeName] = struct{}{}
		customActions = append(customActions, customRouteAction{
			Name:       action,
			MethodName: naming.ToPascalCase(action),
			RouteName:  routeName,
			Path:       naming.ToKebabCase(routeName),
		})
	}
	return customActions
}

func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		service: templates.GetGlobalTemplateService(),
	}
}

func (tr *TemplateRenderer) RenderControllerFile(controller *GeneratedController, diMode, inertia string) (string, error) {
	if controller.ModelName == "" {
		controller.ModelName = controller.ResourceName
	}
	if controller.ModelPluralName == "" {
		controller.ModelPluralName = controller.PluralName
	}
	if controller.ModelPluralResourceName == "" {
		controller.ModelPluralResourceName = controller.PluralResourceName
	}

	var templateName string
	switch controller.Type {
	case ResourceController:
		templateName = "resource_controller.tmpl"
		if inertia == "vue" {
			templateName = "inertia_vue_resource_controller.tmpl"
			if diMode == "uberfx" {
				templateName = "inertia_vue_resource_controller_fx.tmpl"
			}
		} else if diMode == "uberfx" {
			templateName = "resource_controller_fx.tmpl"
		}
	case ResourceControllerNoViews:
		templateName = "resource_controller_no_views.tmpl"
		if diMode == "uberfx" {
			templateName = "resource_controller_no_views_fx.tmpl"
		}
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
		"HasAction": func(action string) bool {
			if len(controller.Actions) == 0 {
				return true
			}
			return slices.Contains(controller.Actions, action)
		},
		"CustomActions": func() []customRouteAction {
			return customRouteActions(controller.Actions)
		},
		"InertiaDataType":  inertiaDataType,
		"InertiaDataValue": inertiaDataValue,
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

func inertiaDataType(field GeneratedField) string {
	switch field.GoType {
	case "sql.NullString", "bun.NullString", "json.RawMessage", "*json.RawMessage", "[]byte":
		return "string"
	case "sql.NullBool", "bun.NullBool":
		return "bool"
	case "sql.NullInt16":
		return "int16"
	case "sql.NullInt32", "bun.NullInt32":
		return "int32"
	case "sql.NullInt64", "bun.NullInt64":
		return "int64"
	case "sql.NullFloat64", "bun.NullFloat64":
		return "float64"
	case "sql.NullTime", "bun.NullTime":
		return "time.Time"
	}

	if strings.HasPrefix(field.GoType, "*") {
		return strings.TrimPrefix(field.GoType, "*")
	}

	return field.GoType
}

func inertiaDataValue(field GeneratedField, source string) string {
	switch field.GoType {
	case "sql.NullString", "bun.NullString":
		return source + ".String"
	case "sql.NullBool", "bun.NullBool":
		return source + ".Bool"
	case "sql.NullInt16":
		return source + ".Int16"
	case "sql.NullInt32", "bun.NullInt32":
		return source + ".Int32"
	case "sql.NullInt64", "bun.NullInt64":
		return source + ".Int64"
	case "sql.NullFloat64", "bun.NullFloat64":
		return source + ".Float64"
	case "sql.NullTime", "bun.NullTime":
		return source + ".Time"
	case "json.RawMessage":
		return "string(" + source + ")"
	case "*json.RawMessage":
		return "func() string { if " + source + " == nil { return \"\" }; return string(*" + source + ") }()"
	case "[]byte":
		return "string(" + source + ")"
	}

	if strings.HasPrefix(field.GoType, "*") {
		dataType := inertiaDataType(field)
		return "func() " + dataType + " { if " + source + " == nil { return " +
			inertiaZeroValue(dataType) + " }; return *" + source + " }()"
	}

	return source
}

func inertiaZeroValue(goType string) string {
	switch goType {
	case "string":
		return `""`
	case "bool":
		return "false"
	case "int", "int16", "int32", "int64", "float32", "float64":
		return "0"
	default:
		if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") {
			return "nil"
		}
		return goType + "{}"
	}
}

func (tr *TemplateRenderer) generateRouteContent(resourceName, pluralName, idType string, actions []string) (string, error) {
	// Get module path
	modulePath, err := tr.getModulePath()
	if err != nil {
		return "", fmt.Errorf("failed to get module path: %w", err)
	}

	// Create custom data structure for route template (router/routes/users.go)
	data := struct {
		ResourceName  string
		PluralName    string
		ModulePath    string
		IDType        string
		Actions       []string
		CustomActions []customRouteAction
	}{
		ResourceName:  resourceName,
		PluralName:    pluralName,
		ModulePath:    modulePath,
		IDType:        idType,
		Actions:       actions,
		CustomActions: customRouteActions(actions),
	}

	customFuncs := template.FuncMap{
		"HasAction": func(action string) bool {
			if len(actions) == 0 {
				return true
			}
			return slices.Contains(actions, action)
		},
		"kebab": naming.ToKebabCase,
	}

	result, err := tr.service.RenderTemplateWithCustomFunctions("route.tmpl", data, customFuncs)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render route", "route.tmpl")
	}
	return result, nil
}

func (tr *TemplateRenderer) generateRouteRegistrationFile(resourceName, pluralName string, actions []string) (string, error) {
	capitalizedPluralName := naming.Capitalize(naming.ToCamelCase(pluralName))
	lowercasePluralName := naming.ToLowerCamelCaseFromAny(pluralName)

	// Get module path
	modulePath, err := tr.getModulePath()
	if err != nil {
		return "", fmt.Errorf("failed to get module path: %w", err)
	}

	// Create custom data structure for route registration template
	data := struct {
		ResourceName          string
		PluralName            string
		CapitalizedPluralName string
		LowercasePluralName   string
		LowercaseResourceName string
		ModulePath            string
		Actions               []string
		CustomActions         []customRouteAction
	}{
		ResourceName:          resourceName,
		PluralName:            pluralName,
		CapitalizedPluralName: capitalizedPluralName,
		LowercasePluralName:   lowercasePluralName,
		LowercaseResourceName: naming.ToLowerCamelCase(resourceName),
		ModulePath:            modulePath,
		Actions:               actions,
		CustomActions:         customRouteActions(actions),
	}
	customFuncs := template.FuncMap{
		"HasAction": func(action string) bool {
			if len(actions) == 0 {
				return true
			}
			return slices.Contains(actions, action)
		},
	}

	result, err := tr.service.RenderTemplateWithCustomFunctions("route_registration.tmpl", data, customFuncs)
	if err != nil {
		return "", errors.WrapTemplateError(err, "render route registration", "route_registration.tmpl")
	}
	return result, nil
}

// getModulePath reads go.mod to get the module path
func (tr *TemplateRenderer) getModulePath() (string, error) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

package controllers

import (
	"fmt"
	"os"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/templates"
)

const returnErrsMarker = "return errors.Join(errs...)"

// ActionInjector handles injecting action code into existing controller,
// route, and connect files.
type ActionInjector struct {
	templateService *templates.TemplateService
}

// NewActionInjector creates a new ActionInjector.
func NewActionInjector() *ActionInjector {
	return &ActionInjector{
		templateService: templates.GetGlobalTemplateService(),
	}
}

// ActionMethodData holds data for rendering the action_method.tmpl template.
type ActionMethodData struct {
	ReceiverName       string
	PluralResourceName string
	MethodName         string
}

// ActionRouteData holds data for rendering the action_route.tmpl template.
type ActionRouteData struct {
	ResourceName    string
	Namespace       string
	NamespacePascal string
	MethodName      string
	ConstructorName string
	ParamsStruct    string // non-empty for RouteWithSlugs: the generated params struct definition
	Path            string
	PluralName      string
	LowerMethodName string
}

// ActionRegistrationData holds data for rendering the action_registration.tmpl template.
type ActionRegistrationData struct {
	ResourceName    string
	Namespace       string
	NamespacePascal string
	MethodName      string
	HTTPMethod      string
	HandlerVar      string
}

// InjectControllerMethod appends a method stub to the end of the controller file.
// It checks for duplicate methods before injecting.
func (ai *ActionInjector) InjectControllerMethod(controllerPath string, data ActionMethodData) error {
	content, err := os.ReadFile(controllerPath)
	if err != nil {
		return fmt.Errorf("failed to read controller file: %w", err)
	}

	contentStr := string(content)

	// Check for duplicate method
	methodSig := fmt.Sprintf(") %s(etx *echo.Context)", data.MethodName)
	if strings.Contains(contentStr, methodSig) {
		return fmt.Errorf("method %s already exists in %s", data.MethodName, controllerPath)
	}

	// Render the template
	rendered, err := ai.templateService.RenderTemplate("action_method.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render action method template: %w", err)
	}

	// Append to end of file
	newContent := strings.TrimRight(contentStr, "\n") + "\n" + rendered

	if err := os.WriteFile(controllerPath, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("failed to write controller file: %w", err)
	}

	return files.FormatGoFile(controllerPath)
}

// InjectRouteVariable appends a route variable declaration to the end of the routes file.
// It checks for duplicate route variables before injecting.
func (ai *ActionInjector) InjectRouteVariable(routesPath string, data ActionRouteData) error {
	content, err := os.ReadFile(routesPath)
	if err != nil {
		return fmt.Errorf("failed to read routes file: %w", err)
	}

	contentStr := string(content)

	// Check for duplicate route variable
	varName := fmt.Sprintf("var %s%s%s ", data.NamespacePascal, data.ResourceName, data.MethodName)
	if strings.Contains(contentStr, varName) {
		return fmt.Errorf("route variable %s%s%s already exists in %s", data.NamespacePascal, data.ResourceName, data.MethodName, routesPath)
	}

	// Render the template
	rendered, err := ai.templateService.RenderTemplate("action_route.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render action route template: %w", err)
	}

	// Append to end of file
	newContent := strings.TrimRight(contentStr, "\n") + "\n" + rendered

	if err := os.WriteFile(routesPath, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}

	return files.FormatGoFile(routesPath)
}

// InjectRouteRegistration inserts a route registration block before the
// "return errors.Join(errs...)" marker in the connect file.
// If the marker is not found, it prints manual instructions.
func (ai *ActionInjector) InjectRouteRegistration(connectPath string, data ActionRegistrationData) error {
	content, err := os.ReadFile(connectPath)
	if err != nil {
		return fmt.Errorf("failed to read connect file: %w", err)
	}

	contentStr := string(content)

	// Check for duplicate registration
	handlerRef := fmt.Sprintf("Handler: %s.%s,", data.HandlerVar, data.MethodName)
	if strings.Contains(contentStr, handlerRef) {
		return fmt.Errorf("route registration for %s.%s already exists in %s", data.HandlerVar, data.MethodName, connectPath)
	}

	// Find the marker
	if !strings.Contains(contentStr, returnErrsMarker) {
		ai.printManualRegistrationInstructions(data)
		return nil
	}

	// Render the template
	rendered, err := ai.templateService.RenderTemplate("action_registration.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render action registration template: %w", err)
	}

	// Insert before marker
	newContent := strings.Replace(contentStr, returnErrsMarker, rendered+"\n\t"+returnErrsMarker, 1)

	if err := os.WriteFile(connectPath, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("failed to write connect file: %w", err)
	}

	return files.FormatGoFile(connectPath)
}

func (ai *ActionInjector) printManualRegistrationInstructions(data ActionRegistrationData) {
	fmt.Printf(`
INFO: Could not find marker "%s" in connect file.
Add the following route registration manually:

	_, err = r.e.AddRoute(echo.Route{
		Method:  http.Method%s,
		Path:    routes.%s%s%s.Path(),
		Name:    routes.%s%s%s.Name(),
		Handler: %s.%s,
	})
	if err != nil {
		errs = append(errs, err)
	}

`, returnErrsMarker, data.HTTPMethod, data.NamespacePascal, data.ResourceName, data.MethodName, data.NamespacePascal, data.ResourceName, data.MethodName, data.HandlerVar, data.MethodName)
}

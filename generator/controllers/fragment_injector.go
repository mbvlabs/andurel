package controllers

import (
	"fmt"
	"os"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/templates"
)

const returnErrsMarker = "return errors.Join(errs...)"

// FragmentInjector handles injecting fragment code into existing controller,
// route, and connect files.
type FragmentInjector struct {
	templateService *templates.TemplateService
}

// NewFragmentInjector creates a new FragmentInjector.
func NewFragmentInjector() *FragmentInjector {
	return &FragmentInjector{
		templateService: templates.GetGlobalTemplateService(),
	}
}

// FragmentMethodData holds data for rendering the fragment_method.tmpl template.
type FragmentMethodData struct {
	ReceiverName     string
	PluralResourceName string
	MethodName       string
}

// FragmentRouteData holds data for rendering the fragment_route.tmpl template.
type FragmentRouteData struct {
	ResourceName    string
	MethodName      string
	ConstructorName string
	Path            string
	PluralName      string
	LowerMethodName string
}

// FragmentRegistrationData holds data for rendering the fragment_route_registration.tmpl template.
type FragmentRegistrationData struct {
	ResourceName string
	MethodName   string
	HTTPMethod   string
	HandlerVar   string
}

// InjectControllerMethod appends a method stub to the end of the controller file.
// It checks for duplicate methods before injecting.
func (fi *FragmentInjector) InjectControllerMethod(controllerPath string, data FragmentMethodData) error {
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
	rendered, err := fi.templateService.RenderTemplate("fragment_method.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render fragment method template: %w", err)
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
func (fi *FragmentInjector) InjectRouteVariable(routesPath string, data FragmentRouteData) error {
	content, err := os.ReadFile(routesPath)
	if err != nil {
		return fmt.Errorf("failed to read routes file: %w", err)
	}

	contentStr := string(content)

	// Check for duplicate route variable
	varName := fmt.Sprintf("var %s%s ", data.ResourceName, data.MethodName)
	if strings.Contains(contentStr, varName) {
		return fmt.Errorf("route variable %s%s already exists in %s", data.ResourceName, data.MethodName, routesPath)
	}

	// Render the template
	rendered, err := fi.templateService.RenderTemplate("fragment_route.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render fragment route template: %w", err)
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
func (fi *FragmentInjector) InjectRouteRegistration(connectPath string, data FragmentRegistrationData) error {
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
		fi.printManualRegistrationInstructions(data)
		return nil
	}

	// Render the template
	rendered, err := fi.templateService.RenderTemplate("fragment_route_registration.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render fragment route registration template: %w", err)
	}

	// Insert before marker
	newContent := strings.Replace(contentStr, returnErrsMarker, rendered+"\n\t"+returnErrsMarker, 1)

	if err := os.WriteFile(connectPath, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("failed to write connect file: %w", err)
	}

	return files.FormatGoFile(connectPath)
}

func (fi *FragmentInjector) printManualRegistrationInstructions(data FragmentRegistrationData) {
	fmt.Printf(`
INFO: Could not find marker "%s" in connect file.
Add the following route registration manually:

	_, err = r.e.AddRoute(echo.Route{
		Method:  http.Method%s,
		Path:    routes.%s%s.Path(),
		Name:    routes.%s%s.Name(),
		Handler: %s.%s,
	})
	if err != nil {
		errs = append(errs, err)
	}

`, returnErrsMarker, data.HTTPMethod, data.ResourceName, data.MethodName, data.ResourceName, data.MethodName, data.HandlerVar, data.MethodName)
}

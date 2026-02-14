package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// FragmentConfig holds the input configuration for fragment generation.
type FragmentConfig struct {
	ControllerName string // PascalCase, e.g. "Webhook"
	MethodName     string // PascalCase, e.g. "Validate"
	Path           string // Route path, e.g. "/validate" or "/:id/approve"
	HTTPMethod     string // HTTP method, e.g. "GET", "POST"
}

// FragmentManager orchestrates fragment generation across controller,
// routes, and connect files.
type FragmentManager struct {
	injector *controllers.FragmentInjector
}

// NewFragmentManager creates a new FragmentManager.
func NewFragmentManager() *FragmentManager {
	return &FragmentManager{
		injector: controllers.NewFragmentInjector(),
	}
}

// GenerateFragment validates inputs, resolves naming, and delegates to
// FragmentInjector for the three file modifications.
func (fm *FragmentManager) GenerateFragment(config FragmentConfig) error {
	if err := fm.validateConfig(config); err != nil {
		return err
	}

	// Resolve naming
	pluralName := naming.DeriveTableName(config.ControllerName) // e.g. "webhooks"
	receiverName := naming.ToReceiverName(config.ControllerName)
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName)) // e.g. "Webhooks"
	lowercaseResourceName := naming.ToLowerCamelCase(config.ControllerName)

	// Resolve file paths
	controllerPath := filepath.Join("controllers", pluralName+".go")
	routesPath := filepath.Join("router", "routes", pluralName+".go")
	connectPath := filepath.Join("router", "connect_"+pluralName+"_routes.go")

	// Verify all three target files exist before modifying any
	for _, path := range []string{controllerPath, routesPath, connectPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("required file %s does not exist. Generate the controller first: andurel generate controller %s", path, config.ControllerName)
		}
	}

	// Check for duplicates across all files before modifying any
	if err := fm.checkDuplicates(controllerPath, routesPath, connectPath, config); err != nil {
		return err
	}

	// Detect route type from path
	routeType := controllers.DetectRouteType(config.Path)

	// Detect ID type from existing routes file
	idType := fm.detectIDTypeFromRoutes(routesPath)

	// Normalize HTTP method for template
	httpMethod := fm.normalizeHTTPMethodName(config.HTTPMethod)

	// 1. Inject controller method
	methodData := controllers.FragmentMethodData{
		ReceiverName:       receiverName,
		PluralResourceName: capitalizedPlural,
		MethodName:         config.MethodName,
	}
	if err := fm.injector.InjectControllerMethod(controllerPath, methodData); err != nil {
		return fmt.Errorf("failed to inject controller method: %w", err)
	}

	// 2. Inject route variable
	routeData := controllers.FragmentRouteData{
		ResourceName:    config.ControllerName,
		MethodName:      config.MethodName,
		ConstructorName: routeType.ConstructorName(idType),
		Path:            config.Path,
		PluralName:      pluralName,
		LowerMethodName: strings.ToLower(config.MethodName),
	}
	if err := fm.injector.InjectRouteVariable(routesPath, routeData); err != nil {
		return fmt.Errorf("failed to inject route variable: %w", err)
	}

	// 3. Inject route registration
	registrationData := controllers.FragmentRegistrationData{
		ResourceName: config.ControllerName,
		MethodName:   config.MethodName,
		HTTPMethod:   httpMethod,
		HandlerVar:   lowercaseResourceName,
	}
	if err := fm.injector.InjectRouteRegistration(connectPath, registrationData); err != nil {
		return fmt.Errorf("failed to inject route registration: %w", err)
	}

	fmt.Printf("Successfully generated fragment %s.%s\n", config.ControllerName, config.MethodName)
	return nil
}

// detectIDTypeFromRoutes scans an existing routes file for ID route constructors
// and returns the corresponding Go type string. Defaults to "uuid.UUID".
func (fm *FragmentManager) detectIDTypeFromRoutes(routesPath string) string {
	content, err := os.ReadFile(routesPath)
	if err != nil {
		return "uuid.UUID"
	}
	s := string(content)
	switch {
	case strings.Contains(s, "NewRouteWithSerialID"):
		return "int32"
	case strings.Contains(s, "NewRouteWithBigSerialID"):
		return "int64"
	case strings.Contains(s, "NewRouteWithStringID"):
		return "string"
	default:
		return "uuid.UUID"
	}
}

func (fm *FragmentManager) validateConfig(config FragmentConfig) error {
	// Validate controller name is PascalCase
	pascalRegex, err := regexp.Compile(`^[A-Z][a-zA-Z0-9]*$`)
	if err != nil {
		return fmt.Errorf("failed to compile PascalCase pattern: %w", err)
	}
	if !pascalRegex.MatchString(config.ControllerName) {
		return fmt.Errorf("controller name '%s' must be PascalCase (e.g. Webhook, Article)", config.ControllerName)
	}

	// Validate method name is PascalCase
	if !pascalRegex.MatchString(config.MethodName) {
		return fmt.Errorf("method name '%s' must be PascalCase (e.g. Validate, ShowBySlug)", config.MethodName)
	}

	// Validate path starts with /
	if !strings.HasPrefix(config.Path, "/") {
		return fmt.Errorf("path '%s' must start with /", config.Path)
	}

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
	}
	if !validMethods[strings.ToUpper(config.HTTPMethod)] {
		return fmt.Errorf("invalid HTTP method '%s'. Must be one of: GET, POST, PUT, DELETE, PATCH", config.HTTPMethod)
	}

	// Validate controller name is singular
	snake := naming.ToSnakeCase(config.ControllerName)
	if inflection.Singular(snake) != snake {
		return fmt.Errorf("controller name '%s' should be singular (e.g. %s)",
			config.ControllerName,
			naming.DeriveResourceName(naming.DeriveTableName(config.ControllerName)),
		)
	}

	return nil
}

func (fm *FragmentManager) checkDuplicates(controllerPath, routesPath, connectPath string, config FragmentConfig) error {
	// Check controller method duplicate
	controllerContent, err := os.ReadFile(controllerPath)
	if err != nil {
		return fmt.Errorf("failed to read controller file: %w", err)
	}
	methodSig := fmt.Sprintf(") %s(etx *echo.Context)", config.MethodName)
	if strings.Contains(string(controllerContent), methodSig) {
		return fmt.Errorf("method %s already exists in %s", config.MethodName, controllerPath)
	}

	// Check route variable duplicate
	routesContent, err := os.ReadFile(routesPath)
	if err != nil {
		return fmt.Errorf("failed to read routes file: %w", err)
	}
	varName := fmt.Sprintf("var %s%s ", config.ControllerName, config.MethodName)
	if strings.Contains(string(routesContent), varName) {
		return fmt.Errorf("route variable %s%s already exists in %s", config.ControllerName, config.MethodName, routesPath)
	}

	// Check route registration duplicate
	connectContent, err := os.ReadFile(connectPath)
	if err != nil {
		return fmt.Errorf("failed to read connect file: %w", err)
	}
	lowercaseResourceName := naming.ToLowerCamelCase(config.ControllerName)
	handlerRef := fmt.Sprintf("Handler: %s.%s,", lowercaseResourceName, config.MethodName)
	if strings.Contains(string(connectContent), handlerRef) {
		return fmt.Errorf("route registration for %s.%s already exists in %s", lowercaseResourceName, config.MethodName, connectPath)
	}

	return nil
}

// normalizeHTTPMethodName converts an HTTP method string to the Go net/http constant suffix.
// e.g. "GET" -> "Get", "POST" -> "Post", "DELETE" -> "Delete"
func (fm *FragmentManager) normalizeHTTPMethodName(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "Get"
	case "POST":
		return "Post"
	case "PUT":
		return "Put"
	case "DELETE":
		return "Delete"
	case "PATCH":
		return "Patch"
	default:
		return "Get"
	}
}

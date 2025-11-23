package controllers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type RouteGenerator struct {
	fileManager      files.Manager
	templateRenderer *TemplateRenderer
}

func NewRouteGenerator() *RouteGenerator {
	return &RouteGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
	}
}

func (rg *RouteGenerator) GenerateRoutes(resourceName, pluralName string) error {
	routesPath := filepath.Join("router/routes", pluralName+".go")

	if _, err := os.Stat(routesPath); err == nil {
		return fmt.Errorf("routes file %s already exists", routesPath)
	}

	routeContent, err := rg.templateRenderer.generateRouteContent(resourceName, pluralName)
	if err != nil {
		return fmt.Errorf("failed to generate route content: %w", err)
	}

	if err := rg.fileManager.EnsureDir("router/routes"); err != nil {
		return err
	}

	if err := os.WriteFile(routesPath, []byte(routeContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}

	if err := rg.formatGoFile(routesPath); err != nil {
		return fmt.Errorf("failed to format routes file: %w", err)
	}

	if err := rg.updateRouterRegister(resourceName, pluralName); err != nil {
		return fmt.Errorf("failed to update router register: %w", err)
	}

	return nil
}

func (rg *RouteGenerator) updateRouterRegister(resourceName, pluralName string) error {
	registryPath := "router/registry.go"
	registerPath := "router/register.go"

	capitalizedPluralName := naming.Capitalize(naming.ToCamelCase(pluralName))
	functionName := fmt.Sprintf("register%sRoutes", capitalizedPluralName)

	// Update router/registry.go with the new addEntry call
	if err := rg.updateRegistryFile(functionName, registryPath); err != nil {
		return err
	}

	// Append the registration function to router/register.go
	if err := rg.appendRegistrationFunction(resourceName, pluralName, registerPath); err != nil {
		return err
	}

	return nil
}

func (rg *RouteGenerator) updateRegistryFile(functionName, registryPath string) error {
	content, err := os.ReadFile(registryPath)
	if err != nil {
		return fmt.Errorf("failed to read router/registry.go: %w", err)
	}

	lines := splitLines(string(content))

	// Find the var registrar line and the last addEntry in the chain
	registrarVarIndex := -1
	lastAddEntryIndex := -1

	for i, line := range lines {
		if contains(line, "var registrar = newRegistrarBuilder()") {
			registrarVarIndex = i
		}
		if registrarVarIndex != -1 && contains(line, "addEntry(") {
			lastAddEntryIndex = i
		}
	}

	if registrarVarIndex == -1 {
		return fmt.Errorf("could not find var registrar in router/registry.go")
	}

	if lastAddEntryIndex == -1 {
		return fmt.Errorf("could not find addEntry in router/registry.go")
	}

	// Insert the new addEntry call after the last one
	// First, check if the last addEntry line ends with a dot
	if !strings.HasSuffix(strings.TrimSpace(lines[lastAddEntryIndex]), ".") {
		// Add a dot to the end of the last addEntry line
		lines[lastAddEntryIndex] = lines[lastAddEntryIndex] + "."
	}

	// Create the new addEntry line (without leading dot)
	newAddEntry := fmt.Sprintf("\taddEntry(%s)", functionName)

	result := append(
		lines[:lastAddEntryIndex+1],
		append([]string{newAddEntry}, lines[lastAddEntryIndex+1:]...)...)

	output := joinLines(result)

	if err := os.WriteFile(registryPath, []byte(output), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write router/registry.go: %w", err)
	}

	return rg.formatGoFile(registryPath)
}

func (rg *RouteGenerator) appendRegistrationFunction(resourceName, pluralName, registerPath string) error {
	// Generate the registration function
	routeRegistrationFunc, err := rg.templateRenderer.generateRouteRegistrationFunction(resourceName, pluralName)
	if err != nil {
		return fmt.Errorf("failed to generate route registration function: %w", err)
	}

	// Read existing content
	content, err := os.ReadFile(registerPath)
	if err != nil {
		return fmt.Errorf("failed to read router/register.go: %w", err)
	}

	// Append the new function
	newContent := string(content) + "\n" + routeRegistrationFunc + "\n"

	if err := os.WriteFile(registerPath, []byte(newContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write router/register.go: %w", err)
	}

	return rg.formatGoFile(registerPath)
}

func (rg *RouteGenerator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

func splitLines(s string) []string {
	lines := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

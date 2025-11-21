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
	registerPath := "router/register.go"

	content, err := os.ReadFile(registerPath)
	if err != nil {
		return fmt.Errorf("failed to read router/register.go: %w", err)
	}

	lines := []string{}
	lines = append(lines, splitLines(string(content))...)

	routeNotFoundIndex := -1
	closingBraceIndex := -1

	for i, line := range lines {
		if contains(line, "handler.RouteNotFound") {
			routeNotFoundIndex = i
		}
		if routeNotFoundIndex != -1 && strings.TrimSpace(line) == "}" {
			closingBraceIndex = i
			break
		}
	}

	if routeNotFoundIndex == -1 {
		return fmt.Errorf("could not find RouteNotFound in router/register.go")
	}

	if closingBraceIndex == -1 {
		return fmt.Errorf("could not find closing brace of registrar function")
	}

	capitalizedPluralName := naming.Capitalize(naming.ToCamelCase(pluralName))
	functionName := fmt.Sprintf("register%sRoutes", capitalizedPluralName)

	functionCall := []string{
		"",
		fmt.Sprintf("\t%s(handler, ctrls)", functionName),
	}

	result := append(
		lines[:closingBraceIndex],
		append(functionCall, lines[closingBraceIndex:]...)...)

	registerFunction := []string{
		"",
		fmt.Sprintf("func %s(handler *echo.Echo, ctrls controllers.Controllers) {", functionName),
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodGet, routes.%sIndex.Path(), ctrls.%s.Index,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sIndex.Name()", resourceName),
		"",
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodGet, routes.%sShow.Path(), ctrls.%s.Show,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sShow.Name()", resourceName),
		"",
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodGet, routes.%sNew.Path(), ctrls.%s.New,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sNew.Name()", resourceName),
		"",
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodPost, routes.%sCreate.Path(), ctrls.%s.Create,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sCreate.Name()", resourceName),
		"",
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodGet, routes.%sEdit.Path(), ctrls.%s.Edit,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sEdit.Name()", resourceName),
		"",
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodPut, routes.%sUpdate.Path(), ctrls.%s.Update,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sUpdate.Name()", resourceName),
		"",
		"\thandler.Add(",
		fmt.Sprintf(
			"\t\thttp.MethodDelete, routes.%sDestroy.Path(), ctrls.%s.Destroy,",
			resourceName,
			capitalizedPluralName,
		),
		fmt.Sprintf("\t).Name = routes.%sDestroy.Name()", resourceName),
		"}",
	}

	result = append(result, registerFunction...)

	output := joinLines(result)

	if err := os.WriteFile(registerPath, []byte(output), constants.FilePermissionPrivate); err != nil {
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

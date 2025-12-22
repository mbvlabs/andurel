package controllers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
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

	if err := rg.createRouteRegistrationFile(resourceName, pluralName); err != nil {
		return fmt.Errorf("failed to create route registration file: %w", err)
	}

	return nil
}

func (rg *RouteGenerator) createRouteRegistrationFile(resourceName, pluralName string) error {
	connectPath := filepath.Join("router", "connect_"+pluralName+"_routes.go")

	if _, err := os.Stat(connectPath); err == nil {
		return fmt.Errorf("route registration file %s already exists", connectPath)
	}

	// Generate the registration file content
	registrationContent, err := rg.templateRenderer.generateRouteRegistrationFile(resourceName, pluralName)
	if err != nil {
		return fmt.Errorf("failed to generate route registration content: %w", err)
	}

	if err := rg.fileManager.EnsureDir("router"); err != nil {
		return err
	}

	if err := os.WriteFile(connectPath, []byte(registrationContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write route registration file: %w", err)
	}

	return rg.formatGoFile(connectPath)
}

func (rg *RouteGenerator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

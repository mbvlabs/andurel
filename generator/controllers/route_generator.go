package controllers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
)

type RouteGenerator struct {
	fileManager      files.FileManager
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

	return rg.registerRoutes(resourceName)
}

func (rg *RouteGenerator) registerRoutes(resourceName string) error {
	routesFilePath := "router/routes/routes.go"

	content, err := os.ReadFile(routesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read routes.go: %w", err)
	}

	contentStr := string(content)

	routeSliceName := resourceName + "Routes"

	if strings.Contains(contentStr, routeSliceName) {
		return nil
	}

	lines := strings.Split(contentStr, "\n")
	var modifiedLines []string
	added := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "return r" && !added {
			modifiedLines = append(modifiedLines, "")
			modifiedLines = append(modifiedLines, "\tr = append(")
			modifiedLines = append(modifiedLines, "\t\tr,")
			modifiedLines = append(modifiedLines, fmt.Sprintf("\t\t%s...,", routeSliceName))
			modifiedLines = append(modifiedLines, "\t)")
			modifiedLines = append(modifiedLines, "")
			added = true
		}
		modifiedLines = append(modifiedLines, line)
	}

	if !added {
		return fmt.Errorf("could not find appropriate place to register routes")
	}

	if err := os.WriteFile(routesFilePath, []byte(strings.Join(modifiedLines, "\n")), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write modified routes file: %w", err)
	}

	return rg.formatGoFile(routesFilePath)
}

func (rg *RouteGenerator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

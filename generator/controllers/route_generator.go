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

	return rg.registerRoutes(resourceName)
}

func (rg *RouteGenerator) registerRoutes(resourceName string) error {
	routesFilePath := "router/routes/routes.go"

	content, err := os.ReadFile(routesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read routes.go: %w", err)
	}

	contentStr := string(content)

	routeIdentifier := resourceName + "Index"

	if strings.Contains(contentStr, routeIdentifier) {
		return nil
	}

	lines := strings.Split(contentStr, "\n")
	insertIdx := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "HomePage," {
			insertIdx = i + 1
			break
		}
	}

	if insertIdx == len(lines) {
		for i, line := range lines {
			if strings.TrimSpace(line) == "}" {
				insertIdx = i
				break
			}
		}
	}

	if insertIdx == len(lines) {
		return fmt.Errorf("could not determine insertion point for generated routes")
	}

	routeEntries := []string{
		fmt.Sprintf("\t%sIndex,", resourceName),
		fmt.Sprintf("\t%sShow.Route,", resourceName),
		fmt.Sprintf("\t%sNew,", resourceName),
		fmt.Sprintf("\t%sCreate,", resourceName),
		fmt.Sprintf("\t%sEdit.Route,", resourceName),
		fmt.Sprintf("\t%sUpdate.Route,", resourceName),
		fmt.Sprintf("\t%sDestroy.Route,", resourceName),
	}

	block := append([]string{""}, routeEntries...)

	updatedLines := append([]string{}, lines[:insertIdx]...)
	updatedLines = append(updatedLines, block...)
	updatedLines = append(updatedLines, lines[insertIdx:]...)

	if err := os.WriteFile(routesFilePath, []byte(strings.Join(updatedLines, "\n")), constants.FilePermissionPrivate); err != nil {
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

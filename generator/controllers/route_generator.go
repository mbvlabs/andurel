package controllers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type RouteGenerator struct {
	fileManager      files.Manager
	templateRenderer *TemplateRenderer
	mainInjector     *MainInjector
}

func NewRouteGenerator() *RouteGenerator {
	return &RouteGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
		mainInjector:     NewMainInjector(),
	}
}

func (rg *RouteGenerator) GenerateRoutes(resourceName, pluralName, idType, diMode string) error {
	return rg.GenerateRoutesWithActions(resourceName, pluralName, idType, diMode, nil)
}

func (rg *RouteGenerator) GenerateRoutesWithActions(resourceName, pluralName, idType, diMode string, actions []string) error {
	return rg.GenerateRoutesWithActionsAndConstructor(resourceName, pluralName, idType, diMode, actions, true)
}

func (rg *RouteGenerator) GenerateRoutesWithActionsAndConstructor(resourceName, pluralName, idType, diMode string, actions []string, withDB bool) error {
	routesPath := filepath.Join("router/routes", pluralName+".go")

	if _, err := os.Stat(routesPath); err == nil {
		if len(actions) > 0 {
			existingActions, err := existingRouteFileActions(routesPath, resourceName, pluralName)
			if err != nil {
				return err
			}
			actions = mergeActions(existingActions, actions)
			routeContent, err := rg.templateRenderer.generateRouteContent(resourceName, pluralName, idType, actions)
			if err != nil {
				return fmt.Errorf("failed to generate route content: %w", err)
			}

			if err := os.WriteFile(routesPath, []byte(routeContent), constants.FilePermissionPrivate); err != nil {
				return fmt.Errorf("failed to write routes file: %w", err)
			}

			if err := files.FormatGoFile(routesPath); err != nil {
				return fmt.Errorf("failed to format routes file: %w", err)
			}
		}

		if diMode != "uberfx" {
			if err := rg.createRouteRegistrationFile(resourceName, pluralName, actions); err != nil {
				return fmt.Errorf("failed to create route registration file: %w", err)
			}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat routes file %s: %w", routesPath, err)
	}

	routeContent, err := rg.templateRenderer.generateRouteContent(resourceName, pluralName, idType, actions)
	if err != nil {
		return fmt.Errorf("failed to generate route content: %w", err)
	}

	if err := rg.fileManager.EnsureDir("router/routes"); err != nil {
		return err
	}

	if err := os.WriteFile(routesPath, []byte(routeContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}

	if err := files.FormatGoFile(routesPath); err != nil {
		return fmt.Errorf("failed to format routes file: %w", err)
	}

	if diMode == "uberfx" {
		if err := rg.mainInjector.InjectFXController(resourceName, pluralName); err != nil {
			slog.Warn("unexpected error injecting fx controller", "error", err)
		}
	} else {
		if err := rg.createRouteRegistrationFile(resourceName, pluralName, actions); err != nil {
			return fmt.Errorf("failed to create route registration file: %w", err)
		}

		// Inject into cmd/app/main.go
		if err := rg.mainInjector.InjectControllerWithDB(resourceName, pluralName, withDB); err != nil {
			slog.Warn("unexpected error injecting controller", "error", err)
		}
	}

	return nil
}

func (rg *RouteGenerator) createRouteRegistrationFile(resourceName, pluralName string, actions []string) error {
	connectPath := filepath.Join("router", "connect_"+pluralName+"_routes.go")

	if _, err := os.Stat(connectPath); err == nil {
		existingActions, err := existingRouteRegistrationActions(connectPath, resourceName)
		if err != nil {
			return err
		}
		actions = mergeActions(existingActions, actions)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat route registration file %s: %w", connectPath, err)
	}

	// Generate the registration file content
	registrationContent, err := rg.templateRenderer.generateRouteRegistrationFile(resourceName, pluralName, actions)
	if err != nil {
		return fmt.Errorf("failed to generate route registration content: %w", err)
	}

	if err := rg.fileManager.EnsureDir("router"); err != nil {
		return err
	}

	if err := os.WriteFile(connectPath, []byte(registrationContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write route registration file: %w", err)
	}

	return files.FormatGoFile(connectPath)
}

func existingRouteRegistrationActions(connectPath, resourceName string) ([]string, error) {
	content, err := os.ReadFile(connectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read route registration file %s: %w", connectPath, err)
	}

	actions := make([]string, 0, len(crudActions))
	for _, action := range crudActions {
		handler := fmt.Sprintf(".%s,", naming.ToPascalCase(action))
		if strings.Contains(string(content), handler) && !slices.Contains(actions, action) {
			actions = append(actions, action)
			continue
		}

		routeRef := fmt.Sprintf("routes.%s%s.", resourceName, naming.ToPascalCase(action))
		if strings.Contains(string(content), routeRef) && !slices.Contains(actions, action) {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

func existingRouteFileActions(routesPath, resourceName, pluralName string) ([]string, error) {
	content, err := os.ReadFile(routesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes file %s: %w", routesPath, err)
	}

	contentStr := string(content)
	actions := make([]string, 0, len(crudActions))
	for _, action := range crudActions {
		routeVar := fmt.Sprintf("var %s%s", resourceName, naming.ToPascalCase(action))
		if strings.Contains(contentStr, routeVar) && !slices.Contains(actions, action) {
			actions = append(actions, action)
		}
	}

	routeNamePattern := regexp.MustCompile(fmt.Sprintf(`"%s\.([a-z0-9_]+)"`, regexp.QuoteMeta(pluralName)))
	for _, match := range routeNamePattern.FindAllStringSubmatch(contentStr, -1) {
		if len(match) != 2 {
			continue
		}
		action := match[1]
		if !slices.Contains(actions, action) {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

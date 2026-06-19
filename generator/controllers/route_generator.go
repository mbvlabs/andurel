package controllers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	fxInjector       *FxInjector
}

func NewRouteGenerator() *RouteGenerator {
	return &RouteGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
		mainInjector:     NewMainInjector(),
		fxInjector:       NewFxInjector(),
	}
}

func (rg *RouteGenerator) GenerateRoutes(resourceName, pluralName, idType, diMode string) error {
	return rg.GenerateRoutesWithActions(resourceName, pluralName, idType, diMode, nil)
}

func (rg *RouteGenerator) GenerateRoutesWithActions(resourceName, pluralName, idType, diMode string, actions []string) error {
	routesPath := filepath.Join("router/routes", pluralName+".go")

	if _, err := os.Stat(routesPath); err == nil {
		if len(actions) > 0 {
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
		if err := rg.fxInjector.InjectController(resourceName, pluralName); err != nil {
			slog.Warn("unexpected error injecting fx controller", "error", err)
		}
		if err := rg.mainInjector.InjectFXController(resourceName, pluralName); err != nil {
			slog.Warn("unexpected error injecting fx controller", "error", err)
		}
	} else {
		if err := rg.createRouteRegistrationFile(resourceName, pluralName, actions); err != nil {
			return fmt.Errorf("failed to create route registration file: %w", err)
		}

		// Inject into cmd/app/main.go
		if err := rg.mainInjector.InjectController(resourceName, pluralName); err != nil {
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

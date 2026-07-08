package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// RouteGenerator generates route artifacts.
type RouteGenerator struct {
	fileManager      files.Manager
	templateRenderer *TemplateRenderer
}

// NewRouteGenerator creates a new route generator.
func NewRouteGenerator() *RouteGenerator {
	return &RouteGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
	}
}

// GenerateRoutes performs the generate routes operation.
func (rg *RouteGenerator) GenerateRoutes(resourceName, namespace, pluralName, idType string, actions []string) error {
	prefixedPluralName := namespacePrefix(namespace) + pluralName
	routesPath := filepath.Join("router/routes", prefixedPluralName+".go")

	if _, err := os.Stat(routesPath); err == nil {
		if len(actions) > 0 {
			existingActions, err := existingRouteFileActions(routesPath, resourceName, namespace, pluralName)
			if err != nil {
				return err
			}
			actions = mergeActions(existingActions, actions)
			routeContent, err := rg.templateRenderer.generateRouteContent(resourceName, namespace, pluralName, idType, actions)
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

		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat routes file %s: %w", routesPath, err)
	}

	routeContent, err := rg.templateRenderer.generateRouteContent(resourceName, namespace, pluralName, idType, actions)
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

	return nil
}

func existingRouteFileActions(routesPath, resourceName, namespace, pluralName string) ([]string, error) {
	content, err := os.ReadFile(routesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes file %s: %w", routesPath, err)
	}

	prefix := naming.NamespaceToPascal(namespace)
	contentStr := string(content)
	actions := make([]string, 0, len(crudActions))
	for _, action := range crudActions {
		routeVar := fmt.Sprintf("var %s%s%s", prefix, resourceName, naming.ToPascalCase(action))
		if strings.Contains(contentStr, routeVar) && !slices.Contains(actions, action) {
			actions = append(actions, action)
		}
	}

	routeNamePrefix := pluralName
	if namespace != "" {
		routeNamePrefix = naming.NamespaceRouteName(namespace) + "." + pluralName
	}
	routeNamePattern := regexp.MustCompile(fmt.Sprintf(`"%s\.([a-z0-9_]+)"`, regexp.QuoteMeta(routeNamePrefix)))
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

func namespacePrefix(namespace string) string {
	return naming.NamespaceFilePrefix(namespace)
}

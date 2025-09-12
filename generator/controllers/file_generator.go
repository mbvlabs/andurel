package controllers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/pkg/constants"
)

type FileGenerator struct {
	fileManager      *files.Manager
	templateRenderer *TemplateRenderer
	routeGenerator   *RouteGenerator
}

func NewFileGenerator() *FileGenerator {
	return &FileGenerator{
		fileManager:      files.NewManager(),
		templateRenderer: NewTemplateRenderer(),
		routeGenerator:   NewRouteGenerator(),
	}
}

func (fg *FileGenerator) GenerateController(
	cat *catalog.Catalog,
	resourceName string,
	controllerType ControllerType,
	modulePath string,
	databaseType string,
) error {
	pluralName := inflection.Plural(strings.ToLower(resourceName))
	controllerPath := filepath.Join("controllers", pluralName+".go")

	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	generator := NewGenerator(databaseType)
	controller, err := generator.Build(cat, Config{
		ResourceName:   resourceName,
		PluralName:     pluralName,
		PackageName:    "controllers",
		ModulePath:     modulePath,
		ControllerType: controllerType,
	})
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	controllerContent, err := fg.templateRenderer.RenderControllerFile(controller)
	if err != nil {
		return fmt.Errorf("failed to render controller file: %w", err)
	}

	if err := fg.fileManager.EnsureDirectoryExists("controllers"); err != nil {
		return err
	}

	if err := os.WriteFile(controllerPath, []byte(controllerContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write controller file: %w", err)
	}

	if err := fg.formatGoFile(controllerPath); err != nil {
		return fmt.Errorf("failed to format controller file: %w", err)
	}

	if err := fg.routeGenerator.GenerateRoutes(resourceName, pluralName); err != nil {
		return fmt.Errorf("failed to generate routes: %w", err)
	}

	if err := fg.registerController(resourceName); err != nil {
		return fmt.Errorf("failed to register controller: %w", err)
	}

	return nil
}

func (fg *FileGenerator) registerController(resourceName string) error {
	controllerFilePath := "controllers/controller.go"

	content, err := os.ReadFile(controllerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read controller.go: %w", err)
	}

	contentStr := string(content)

	controllerField := resourceName + "s " + resourceName + "s"
	if strings.Contains(contentStr, controllerField) {
		return nil
	}

	lines := strings.Split(contentStr, "\n")
	var modifiedLines []string

	for _, line := range lines {
		modifiedLines = append(modifiedLines, line)

		if strings.Contains(line, "Pages  Pages") {
			modifiedLines = append(modifiedLines, "\t"+controllerField)
		}

		if strings.Contains(line, "api := newAPI(db)") {
			modifiedLines = append(
				modifiedLines,
				"\t"+strings.ToLower(resourceName)+"s := new"+resourceName+"s(db)",
			)
		}

		if strings.TrimSpace(line) == "pages," {
			modifiedLines = append(modifiedLines, "\t\t"+strings.ToLower(resourceName)+"s,")
		}
	}

	if err := os.WriteFile(controllerFilePath, []byte(strings.Join(modifiedLines, "\n")), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write modified controller file: %w", err)
	}

	return fg.formatGoFile(controllerFilePath)
}

func (fg *FileGenerator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

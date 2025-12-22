package controllers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type FileGenerator struct {
	fileManager      files.Manager
	templateRenderer *TemplateRenderer
	routeGenerator   *RouteGenerator
}

func NewFileGenerator() *FileGenerator {
	return &FileGenerator{
		fileManager:      files.NewUnifiedFileManager(),
		templateRenderer: NewTemplateRenderer(),
		routeGenerator:   NewRouteGenerator(),
	}
}

func (fg *FileGenerator) GenerateController(
	cat *catalog.Catalog,
	resourceName string,
	tableName string,
	controllerType ControllerType,
	modulePath string,
	databaseType string,
) error {
	pluralName := naming.DeriveTableName(resourceName)
	controllerPath := filepath.Join("controllers", tableName+".go")

	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	generator := NewGenerator(databaseType)
	controller, err := generator.Build(cat, Config{
		ResourceName:   resourceName,
		PluralName:     pluralName,
		TableName:      tableName,
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

	if err := fg.fileManager.EnsureDir("controllers"); err != nil {
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

	return nil
}

func (fg *FileGenerator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

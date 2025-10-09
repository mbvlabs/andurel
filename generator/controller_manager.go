package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ControllerManager struct {
	validator           *InputValidator
	projectManager      *ProjectManager
	migrationManager    *MigrationManager
	controllerGenerator *controllers.Generator
	config              *UnifiedConfig
}

func NewControllerManager(
	validator *InputValidator,
	projectManager *ProjectManager,
	migrationManager *MigrationManager,
	controllerGenerator *controllers.Generator,
	config *UnifiedConfig,
) *ControllerManager {
	return &ControllerManager{
		validator:           validator,
		projectManager:      projectManager,
		migrationManager:    migrationManager,
		controllerGenerator: controllerGenerator,
		config:              config,
	}
}

func (c *ControllerManager) GenerateController(resourceName, tableName string, withViews bool) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := c.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceControllerNoViews
	if withViews {
		controllerType = controllers.ResourceController
	}

	if err := c.controllerGenerator.GenerateController(cat, resourceName, controllerType, modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if withViews {
		fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource controller %s (no views)\n", resourceName)
	}

	return nil
}

func (c *ControllerManager) GenerateControllerFromModel(resourceName string, withViews bool) error {
	modulePath, err := c.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate the model first with: andurel generate model %s <table_name>",
			modelPath,
			resourceName,
		)
	}

	if err := c.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	tableName := naming.DeriveTableName(resourceName)

	if err := c.validator.ValidateTableName(tableName); err != nil {
		return fmt.Errorf("derived table name validation failed: %w", err)
	}

	routesFilePath := filepath.Join(c.config.Paths.Routes, "routes.go")
	if _, err := os.Stat(routesFilePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"routes file %s does not exist. Please ensure your project has a routes.go file before generating controllers",
			routesFilePath,
		)
	}

	individualRoutePath := filepath.Join("router/routes", tableName+".go")
	if _, err := os.Stat(individualRoutePath); err == nil {
		return fmt.Errorf("routes file %s already exists", individualRoutePath)
	}

	controllerPath := filepath.Join(c.config.Paths.Controllers, tableName+".go")
	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	controllerFilePath := filepath.Join(c.config.Paths.Controllers, "controller.go")
	if _, err := os.Stat(controllerFilePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"main controller file %s does not exist. Please ensure your project has a controller.go file before generating controllers",
			controllerFilePath,
		)
	}

	content, err := os.ReadFile(controllerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read controller.go: %w", err)
	}

	controllerFieldName := resourceName + "s"
	controllerVarName := naming.ToCamelCase(naming.ToSnakeCase(resourceName)) + "s"
	controllerConstructor := controllerVarName + " := new" + resourceName + "s(db)"
	controllerReturnField := controllerVarName + ","
	contentStr := string(content)
	lines := strings.SplitSeq(contentStr, "\n")

	for line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, controllerFieldName+" ") &&
			strings.HasSuffix(trimmedLine, " "+controllerFieldName) {
			return fmt.Errorf(
				"controller %s is already registered in %s (struct field found)",
				resourceName,
				controllerFilePath,
			)
		}

		if strings.Contains(trimmedLine, controllerConstructor) {
			return fmt.Errorf(
				"controller %s is already registered in %s (constructor call found)",
				resourceName,
				controllerFilePath,
			)
		}

		if trimmedLine == controllerReturnField {
			return fmt.Errorf(
				"controller %s is already registered in %s (return field found)",
				resourceName,
				controllerFilePath,
			)
		}
	}

	if withViews {
		if _, err := os.Stat(c.config.Paths.Views); os.IsNotExist(err) {
			return fmt.Errorf(
				"views directory %s does not exist. Please create the views directory structure before using --with-views",
				c.config.Paths.Views,
			)
		}

		viewPath := filepath.Join(c.config.Paths.Views, tableName+"_resource.templ")
		if _, err := os.Stat(viewPath); err == nil {
			return fmt.Errorf("view file %s already exists", viewPath)
		}
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceControllerNoViews
	if withViews {
		controllerType = controllers.ResourceController
	}

	if err := c.controllerGenerator.GenerateController(cat, resourceName, controllerType, modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if withViews {
		fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource controller %s (no views)\n", resourceName)
	}

	return nil
}

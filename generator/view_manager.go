package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/generator/views"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ViewManager struct {
	validator           *InputValidator
	projectManager      *ProjectManager
	migrationManager    *MigrationManager
	viewGenerator       *views.Generator
	controllerGenerator *controllers.Generator
	config              *UnifiedConfig
}

func NewViewManager(
	validator *InputValidator,
	projectManager *ProjectManager,
	migrationManager    *MigrationManager,
	viewGenerator       *views.Generator,
	controllerGenerator *controllers.Generator,
	config              *UnifiedConfig,
) *ViewManager {
	return &ViewManager{
		validator:           validator,
		projectManager:      projectManager,
		migrationManager:    migrationManager,
		viewGenerator:       viewGenerator,
		controllerGenerator: controllerGenerator,
		config:              config,
	}
}

func (v *ViewManager) GenerateView(resourceName, tableName string) error {
	modulePath, err := v.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	if err := v.validator.ValidateAll(resourceName, tableName, modulePath); err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(v.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := v.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := v.viewGenerator.GenerateView(cat, resourceName, modulePath); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	fmt.Printf("Successfully generated resource view for %s\n", resourceName)

	return nil
}

func (v *ViewManager) GenerateViewFromModel(resourceName string, withController bool) error {
	modulePath, err := v.projectManager.GetModulePath()
	if err != nil {
		return err
	}

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(v.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate the model first with: andurel generate model %s <table_name>",
			modelPath,
			resourceName,
		)
	}

	if err := v.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	tableName := naming.DeriveTableName(resourceName)

	if err := v.validator.ValidateTableName(tableName); err != nil {
		return fmt.Errorf("derived table name validation failed: %w", err)
	}

	if _, err := os.Stat(v.config.Paths.Views); os.IsNotExist(err) {
		return fmt.Errorf(
			"views directory %s does not exist. Please create the views directory structure",
			v.config.Paths.Views,
		)
	}

	viewPath := filepath.Join(v.config.Paths.Views, tableName+"_resource.templ")
	if _, err := os.Stat(viewPath); err == nil {
		return fmt.Errorf("view file %s already exists", viewPath)
	}

	if withController {
		routesFilePath := filepath.Join(v.config.Paths.Routes, "routes.go")
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

		controllerPath := filepath.Join(v.config.Paths.Controllers, tableName+".go")
		if _, err := os.Stat(controllerPath); err == nil {
			return fmt.Errorf("controller file %s already exists", controllerPath)
		}

		controllerFilePath := filepath.Join(v.config.Paths.Controllers, "controller.go")
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
	}

	cat, err := v.migrationManager.BuildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := v.viewGenerator.GenerateViewWithController(cat, resourceName, modulePath, withController); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	if withController {
		controllerType := controllers.ResourceController // with views since we're generating both
		if err := v.controllerGenerator.GenerateController(cat, resourceName, controllerType, modulePath); err != nil {
			return fmt.Errorf("failed to generate controller: %w", err)
		}
		fmt.Printf("Successfully generated resource view for %s with controller\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource view for %s\n", resourceName)
	}

	return nil
}

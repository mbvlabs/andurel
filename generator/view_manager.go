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
	validator        *InputValidator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	viewGenerator    *views.Generator
	config           *UnifiedConfig
}

func NewViewManager(
	validator *InputValidator,
	projectManager *ProjectManager,
	migrationManager *MigrationManager,
	viewGenerator *views.Generator,
	config *UnifiedConfig,
) *ViewManager {
	return &ViewManager{
		validator:        validator,
		projectManager:   projectManager,
		migrationManager: migrationManager,
		viewGenerator:    viewGenerator,
		config:           config,
	}
}

func (v *ViewManager) GenerateView(resourceName, tableName string) error {
	return v.generateView(resourceName, tableName, false)
}

func (v *ViewManager) GenerateViewWithController(resourceName, tableName string) error {
	return v.generateView(resourceName, tableName, true)
}

func (v *ViewManager) generateView(resourceName, tableName string, withController bool) error {
	modulePath := v.projectManager.GetModulePath()

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

	cat, err := v.migrationManager.BuildCatalogFromMigrations(tableName, v.config)
	if err != nil {
		return err
	}

	if err := v.viewGenerator.GenerateViewWithController(cat, resourceName, modulePath, withController); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	fmt.Printf("Successfully generated resource view for %s\n", resourceName)

	return nil
}

func (v *ViewManager) GenerateViewFromModel(resourceName string, withController bool) error {
	modulePath := v.projectManager.GetModulePath()

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
		validationCtx := newControllerValidationContext(resourceName, tableName, v.config)
		if err := validateControllerNotExists(validationCtx); err != nil {
			return err
		}
	}

	cat, err := v.migrationManager.BuildCatalogFromMigrations(tableName, v.config)
	if err != nil {
		return err
	}

	if err := v.viewGenerator.GenerateViewWithController(cat, resourceName, modulePath, withController); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	if withController {
		controllerType := controllers.ResourceController // with views since we're generating both
		fileGen := controllers.NewFileGenerator()
		if err := fileGen.GenerateController(cat, resourceName, controllerType, modulePath, v.config.Database.Type); err != nil {
			return fmt.Errorf("failed to generate controller: %w", err)
		}
		fmt.Printf("Successfully generated resource view for %s with controller\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource view for %s\n", resourceName)
	}

	return nil
}

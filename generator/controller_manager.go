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
	validator        *InputValidator
	projectManager   *ProjectManager
	migrationManager *MigrationManager
	config           *UnifiedConfig
}

func NewControllerManager(
	validator *InputValidator,
	projectManager *ProjectManager,
	migrationManager *MigrationManager,
	config *UnifiedConfig,
) *ControllerManager {
	return &ControllerManager{
		validator:        validator,
		projectManager:   projectManager,
		migrationManager: migrationManager,
		config:           config,
	}
}

func (c *ControllerManager) GenerateController(
	resourceName, tableName string,
	withViews bool,
) error {
	modulePath := c.projectManager.GetModulePath()

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

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName, c.config)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceControllerNoViews
	if withViews {
		controllerType = controllers.ResourceController
	}

	// Check if table name is overridden (different from derived name)
	derivedTableName := naming.DeriveTableName(resourceName)
	tableNameOverridden := tableName != derivedTableName

	fileGen := controllers.NewFileGenerator()
	if err := fileGen.GenerateController(cat, resourceName, tableName, controllerType, modulePath, c.config.Database.Type, tableNameOverridden); err != nil {
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
	modulePath := c.projectManager.GetModulePath()

	var modelFileName strings.Builder
	modelFileName.Grow(len(resourceName) + 3) // +3 for ".go"
	modelFileName.WriteString(naming.ToSnakeCase(resourceName))
	modelFileName.WriteString(".go")
	modelPath := filepath.Join(c.config.Paths.Models, modelFileName.String())
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate the model first: andurel generate model %s",
			modelPath,
			resourceName,
		)
	}

	if err := c.validator.ValidateResourceName(resourceName); err != nil {
		return err
	}

	tableName, tableNameOverridden := ResolveTableNameWithFlag(c.config.Paths.Models, resourceName)

	if tableNameOverridden {
		if err := c.validator.ValidateTableNameOverride(resourceName, tableName); err != nil {
			return fmt.Errorf("table name validation failed: %w", err)
		}
	} else {
		if err := c.validator.ValidateTableName(tableName); err != nil {
			return fmt.Errorf("table name validation failed: %w", err)
		}
	}

	validationCtx := newControllerValidationContext(resourceName, tableName, c.config)
	if err := validateControllerNotExists(validationCtx); err != nil {
		return err
	}

	if withViews {
		if _, err := os.Stat(c.config.Paths.Views); os.IsNotExist(err) {
			return fmt.Errorf(
				"views directory %s does not exist. Create views directory before using --with-views",
				c.config.Paths.Views,
			)
		}

		viewPath := filepath.Join(c.config.Paths.Views, tableName+"_resource.templ")
		if _, err := os.Stat(viewPath); err == nil {
			return fmt.Errorf("view file %s already exists", viewPath)
		}
	}

	cat, err := c.migrationManager.BuildCatalogFromMigrations(tableName, c.config)
	if err != nil {
		return err
	}

	controllerType := controllers.ResourceControllerNoViews
	if withViews {
		controllerType = controllers.ResourceController
	}

	fileGen := controllers.NewFileGenerator()
	if err := fileGen.GenerateController(cat, resourceName, tableName, controllerType, modulePath, c.config.Database.Type, tableNameOverridden); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if withViews {
		fmt.Printf("Successfully generated resource controller %s with views\n", resourceName)
	} else {
		fmt.Printf("Successfully generated resource controller %s (no views)\n", resourceName)
	}

	return nil
}

package generator

import (
	"fmt"

	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/views"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type Coordinator struct {
	modelManager      *ModelManager
	controllerManager *ControllerManager
	viewManager       *ViewManager
	projectManager    *ProjectManager
}

func NewCoordinator() (Coordinator, error) {
	projectManager, err := NewProjectManager()
	if err != nil {
		return Coordinator{}, fmt.Errorf("failed to create project manager: %w", err)
	}

	configManager := NewConfigManager()
	unifiedConfig, err := configManager.Load()
	if err != nil {
		return Coordinator{}, fmt.Errorf("failed to load config: %w", err)
	}

	validator := NewInputValidator()
	fileManager := files.NewUnifiedFileManager()
	migrationManager := NewMigrationManager()

	// Create generators
	modelGenerator := models.NewGenerator(unifiedConfig.Database.Type)
	controllerGenerator := controllers.NewGenerator(unifiedConfig.Database.Type)
	viewGenerator := views.NewGenerator(unifiedConfig.Database.Type)

	// Create managers
	modelManager := NewModelManager(
		validator,
		fileManager,
		modelGenerator,
		projectManager,
		migrationManager,
		unifiedConfig,
	)

	controllerManager := NewControllerManager(
		validator,
		projectManager,
		migrationManager,
		controllerGenerator,
		unifiedConfig,
	)

	viewManager := NewViewManager(
		validator,
		projectManager,
		migrationManager,
		viewGenerator,
		controllerGenerator,
		unifiedConfig,
	)

	return Coordinator{
		modelManager:      modelManager,
		controllerManager: controllerManager,
		viewManager:       viewManager,
		projectManager:    projectManager,
	}, nil
}

func (c *Coordinator) GenerateModel(resourceName string) error {
	return c.modelManager.GenerateModel(resourceName)
}

func (c *Coordinator) GenerateController(resourceName, tableName string, withViews bool) error {
	if err := c.controllerManager.GenerateController(resourceName, tableName, withViews); err != nil {
		return err
	}

	if withViews {
		if err := c.viewManager.GenerateViewWithController(resourceName, tableName); err != nil {
			return err
		}
	}

	return nil
}

func (c *Coordinator) GenerateControllerFromModel(resourceName string, withViews bool) error {
	if err := c.controllerManager.GenerateControllerFromModel(resourceName, withViews); err != nil {
		return err
	}

	if withViews {
		tableName := naming.DeriveTableName(resourceName)
		if err := c.viewManager.GenerateViewWithController(resourceName, tableName); err != nil {
			return err
		}
	}

	return nil
}

func (c *Coordinator) GenerateView(resourceName, tableName string) error {
	return c.viewManager.GenerateView(resourceName, tableName)
}

func (c *Coordinator) GenerateViewFromModel(resourceName string, withController bool) error {
	return c.viewManager.GenerateViewFromModel(resourceName, withController)
}

func (c *Coordinator) RefreshModel(resourceName, tableName string) error {
	return c.modelManager.RefreshModel(resourceName, tableName)
}

func (c *Coordinator) RefreshQueries(resourceName, tableName string) error {
	return c.modelManager.RefreshQueries(resourceName, tableName)
}

func (c *Coordinator) RefreshConstructors(resourceName, tableName string) error {
	return c.modelManager.RefreshConstructors(resourceName, tableName)
}

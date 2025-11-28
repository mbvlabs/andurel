package generator

import (
	"fmt"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/views"
)

type Coordinator struct {
	ModelManager      *ModelManager
	ControllerManager *ControllerManager
	ViewManager       *ViewManager
	projectManager    *ProjectManager
	config            *UnifiedConfig
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
		unifiedConfig,
	)

	viewManager := NewViewManager(
		validator,
		projectManager,
		migrationManager,
		viewGenerator,
		unifiedConfig,
	)

	return Coordinator{
		ModelManager:      modelManager,
		ControllerManager: controllerManager,
		ViewManager:       viewManager,
		projectManager:    projectManager,
		config:            unifiedConfig,
	}, nil
}

// GenerateController coordinates controller and optional view generation
func (c *Coordinator) GenerateController(resourceName, tableName string, withViews bool) error {
	if err := c.ControllerManager.GenerateController(resourceName, tableName, withViews); err != nil {
		return err
	}

	if withViews {
		if err := c.ViewManager.GenerateViewWithController(resourceName, tableName); err != nil {
			return err
		}
	}

	return nil
}

// GenerateControllerFromModel coordinates controller and optional view generation from existing model
func (c *Coordinator) GenerateControllerFromModel(resourceName string, withViews bool) error {
	if err := c.ControllerManager.GenerateControllerFromModel(resourceName, withViews); err != nil {
		return err
	}

	if withViews {
		tableName := ResolveTableName(c.config.Paths.Models, resourceName)
		if err := c.ViewManager.GenerateViewWithController(resourceName, tableName); err != nil {
			return err
		}
	}

	return nil
}

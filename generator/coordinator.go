package generator

import (
	"fmt"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/views"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// Coordinator wires the managers that implement high-level generation workflows.
type Coordinator struct {
	ModelManager      *ModelManager
	ControllerManager *ControllerManager
	ViewManager       *ViewManager
	ActionManager     *ActionManager
	projectManager    *ProjectManager
	config            *UnifiedConfig
}

// NewCoordinator creates a new coordinator.
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

	actionManager := NewActionManager()

	return Coordinator{
		ModelManager:      modelManager,
		ControllerManager: controllerManager,
		ViewManager:       viewManager,
		ActionManager:     actionManager,
		projectManager:    projectManager,
		config:            unifiedConfig,
	}, nil
}

// GenerateController coordinates controller and optional view generation
func (c *Coordinator) GenerateController(resourceName, namespace, tableName string, inertia string, isAPI bool) error {
	return c.GenerateControllerWithActions(resourceName, namespace, tableName, nil, inertia, isAPI)
}

// GenerateControllerWithActions generates a controller and views for selected actions.
func (c *Coordinator) GenerateControllerWithActions(resourceName, namespace, tableName string, actions []string, inertia string, isAPI bool) error {
	return c.GenerateControllerWithActionsForModel(resourceName, namespace, resourceName, tableName, actions, inertia, isAPI)
}

// GenerateControllerWithActionsForModel generates a controller and views when resource and model names differ.
func (c *Coordinator) GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName string, actions []string, inertia string, isAPI bool) error {
	if modelName == "" {
		modelName = resourceName
	}
	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}

	if err := c.ControllerManager.GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName, actions, inertia, isAPI); err != nil {
		return err
	}

	if isAPI {
		return nil
	}

	if err := c.ViewManager.GenerateViewWithControllerActionsForModel(resourceName, modelName, tableName, namespace, actions, inertia); err != nil {
		return err
	}

	return nil
}

// GenerateScaffold coordinates model, controller, and view generation for a
// complete resource scaffold.
func (c *Coordinator) GenerateScaffold(resourceName, namespace, tableName string, skipFactory bool, primaryKeyColumn string, inertia string, isAPI bool) error {
	if primaryKeyColumn != "" {
		if err := c.ModelManager.GenerateModel(resourceName, tableName, skipFactory, primaryKeyColumn); err != nil {
			return err
		}
		c.ControllerManager.SetPrimaryKeyResolver(NopPrimaryKeyResolver{})
	} else {
		if err := c.ModelManager.GenerateModel(resourceName, tableName, skipFactory, ""); err != nil {
			return err
		}
	}

	return c.GenerateController(resourceName, namespace, tableName, inertia, isAPI)
}

// GenerateControllerFromModel coordinates controller and view generation from existing model
func (c *Coordinator) GenerateControllerFromModel(resourceName string) error {
	if err := c.ControllerManager.GenerateControllerFromModel(resourceName); err != nil {
		return err
	}

	tableName := ResolveTableName(c.config.Paths.Models, resourceName)
	if err := c.ViewManager.GenerateViewWithControllerActions(resourceName, tableName, "", nil, ""); err != nil {
		return err
	}

	return nil
}

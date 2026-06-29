package generator

import (
	"fmt"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/views"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type Coordinator struct {
	ModelManager      *ModelManager
	ControllerManager *ControllerManager
	ViewManager       *ViewManager
	FragmentManager   *FragmentManager
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

	fragmentManager := NewFragmentManager()

	return Coordinator{
		ModelManager:      modelManager,
		ControllerManager: controllerManager,
		ViewManager:       viewManager,
		FragmentManager:   fragmentManager,
		projectManager:    projectManager,
		config:            unifiedConfig,
	}, nil
}

// GenerateController coordinates controller and optional view generation
func (c *Coordinator) GenerateController(resourceName, tableName string, withViews bool, inertia string) error {
	return c.GenerateControllerWithActions(resourceName, tableName, withViews, nil, inertia)
}

func (c *Coordinator) GenerateControllerWithActions(resourceName, tableName string, withViews bool, actions []string, inertia string) error {
	return c.GenerateControllerWithActionsForModel(resourceName, resourceName, tableName, withViews, actions, inertia)
}

func (c *Coordinator) GenerateControllerWithActionsForModel(resourceName, modelName, tableName string, withViews bool, actions []string, inertia string) error {
	if modelName == "" {
		modelName = resourceName
	}
	if tableName == "" {
		tableName = naming.DeriveTableName(resourceName)
	}

	if err := c.ControllerManager.GenerateControllerWithActionsForModel(resourceName, modelName, tableName, withViews, actions, inertia); err != nil {
		return err
	}

	if withViews {
		if err := c.ViewManager.GenerateViewWithControllerActionsForModel(resourceName, modelName, tableName, actions, inertia); err != nil {
			return err
		}
	}

	return nil
}

// GenerateScaffold coordinates model, controller, and view generation for a
// complete resource scaffold.
func (c *Coordinator) GenerateScaffold(resourceName, tableName string, skipFactory bool, primaryKeyColumn string, inertia string) error {
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

	return c.GenerateController(resourceName, tableName, true, inertia)
}

// GenerateControllerFromModel coordinates controller and optional view generation from existing model
func (c *Coordinator) GenerateControllerFromModel(resourceName string, withViews bool) error {
	if err := c.ControllerManager.GenerateControllerFromModel(resourceName, withViews); err != nil {
		return err
	}

	if withViews {
		tableName := ResolveTableName(c.config.Paths.Models, resourceName)
		if err := c.ViewManager.GenerateViewWithController(resourceName, tableName, ""); err != nil {
			return err
		}
	}

	return nil
}

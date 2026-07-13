// Package generator orchestrates model, controller, view, and scaffold generation.
package generator

// Generator is the high-level facade for Andurel code generation.
type Generator struct {
	coordinator Coordinator
}

// New creates a generator with the default project managers.
func New() (Generator, error) {
	coordinator, err := NewCoordinator()
	if err != nil {
		return Generator{}, err
	}

	return Generator{
		coordinator: coordinator,
	}, nil
}

// GenerateModel generates a model and optional factory for a resource.
func (g *Generator) GenerateModel(resourceName string, tableNameOverride string, skipFactory bool) error {
	return g.coordinator.ModelManager.GenerateModel(resourceName, tableNameOverride, skipFactory, "")
}

// GenerateModelWithPK generates a model using an explicit primary key column.
func (g *Generator) GenerateModelWithPK(resourceName string, tableNameOverride string, skipFactory bool, primaryKeyColumn string) error {
	return g.coordinator.ModelManager.GenerateModel(resourceName, tableNameOverride, skipFactory, primaryKeyColumn)
}

// GenerateController generates controller and route files for a resource.
func (g *Generator) GenerateController(resourceName, namespace, tableName string, inertia string, isAPI bool) error {
	return g.coordinator.GenerateController(resourceName, namespace, tableName, inertia, isAPI)
}

// GenerateControllerWithActions generates a controller restricted to the requested actions.
func (g *Generator) GenerateControllerWithActions(resourceName, namespace, tableName string, actions []string, inertia string, isAPI bool) error {
	return g.coordinator.GenerateControllerWithActions(resourceName, namespace, tableName, actions, inertia, isAPI)
}

// GenerateControllerWithActionsForModel generates a controller for a distinct model name.
func (g *Generator) GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName string, actions []string, inertia string, isAPI bool) error {
	return g.coordinator.GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName, actions, inertia, isAPI)
}

// GenerateScaffold generates model, factory, controller, routes, and views for a resource.
func (g *Generator) GenerateScaffold(resourceName, namespace, tableName string, skipFactory bool, primaryKeyColumn string, inertia string, isAPI bool) error {
	return g.coordinator.GenerateScaffold(resourceName, namespace, tableName, skipFactory, primaryKeyColumn, inertia, isAPI)
}

// GenerateControllerFromModel generates a controller by reading an existing model.
func (g *Generator) GenerateControllerFromModel(resourceName string) error {
	return g.coordinator.GenerateControllerFromModel(resourceName)
}

// GenerateView generates views for a resource.
func (g *Generator) GenerateView(resourceName, tableName, namespace string) error {
	return g.coordinator.ViewManager.GenerateView(resourceName, tableName, namespace)
}

// GenerateViewFromModel generates views by reading an existing model.
func (g *Generator) GenerateViewFromModel(resourceName string, withController bool) error {
	return g.coordinator.ViewManager.GenerateViewFromModel(resourceName, withController)
}

// SetControllerPKResolver overrides primary key resolution for controller generation.
func (g *Generator) SetControllerPKResolver(resolver PrimaryKeyResolver) {
	g.coordinator.ControllerManager.SetPrimaryKeyResolver(resolver)
}

// GenerateAction adds an action to an existing controller and route set.
func (g *Generator) GenerateAction(config ActionConfig) error {
	return g.coordinator.ActionManager.GenerateAction(config)
}

// GetModulePath returns the current project's Go module path.
func (g *Generator) GetModulePath() string {
	return g.coordinator.projectManager.GetModulePath()
}

// UpdateModel computes the changes needed to refresh an existing model.
func (g *Generator) UpdateModel(resourceName string) (*UpdateModelResult, error) {
	return g.coordinator.ModelManager.UpdateModel(resourceName)
}

// ApplyModelUpdate writes a previously computed model update.
func (g *Generator) ApplyModelUpdate(result *UpdateModelResult) error {
	return g.coordinator.ModelManager.ApplyModelUpdate(result)
}

// SyncFactory refreshes a factory for one resource.
func (g *Generator) SyncFactory(resourceName string, opts FactorySyncOptions) (*FactorySyncResult, error) {
	return g.coordinator.ModelManager.SyncFactory(resourceName, opts)
}

// SyncFactories refreshes factories across the project.
func (g *Generator) SyncFactories(opts FactorySyncOptions) ([]*FactorySyncResult, error) {
	return g.coordinator.ModelManager.SyncFactories(opts)
}

package generator

type Generator struct {
	coordinator Coordinator
}

func New() (Generator, error) {
	coordinator, err := NewCoordinator()
	if err != nil {
		return Generator{}, err
	}

	return Generator{
		coordinator: coordinator,
	}, nil
}

func (g *Generator) GenerateModel(resourceName string, tableNameOverride string, skipFactory bool) error {
	return g.coordinator.ModelManager.GenerateModel(resourceName, tableNameOverride, skipFactory, "")
}

func (g *Generator) GenerateModelWithPK(resourceName string, tableNameOverride string, skipFactory bool, primaryKeyColumn string) error {
	return g.coordinator.ModelManager.GenerateModel(resourceName, tableNameOverride, skipFactory, primaryKeyColumn)
}

func (g *Generator) GenerateController(resourceName, namespace, tableName string, inertia string) error {
	return g.coordinator.GenerateController(resourceName, namespace, tableName, inertia)
}

func (g *Generator) GenerateControllerWithActions(resourceName, namespace, tableName string, actions []string, inertia string) error {
	return g.coordinator.GenerateControllerWithActions(resourceName, namespace, tableName, actions, inertia)
}

func (g *Generator) GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName string, actions []string, inertia string) error {
	return g.coordinator.GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName, actions, inertia)
}

func (g *Generator) GenerateScaffold(resourceName, namespace, tableName string, skipFactory bool, primaryKeyColumn string, inertia string) error {
	return g.coordinator.GenerateScaffold(resourceName, namespace, tableName, skipFactory, primaryKeyColumn, inertia)
}

func (g *Generator) GenerateControllerFromModel(resourceName string) error {
	return g.coordinator.GenerateControllerFromModel(resourceName)
}

func (g *Generator) GenerateView(resourceName, tableName, namespace string) error {
	return g.coordinator.ViewManager.GenerateView(resourceName, tableName, namespace)
}

func (g *Generator) GenerateViewFromModel(resourceName string, withController bool) error {
	return g.coordinator.ViewManager.GenerateViewFromModel(resourceName, withController)
}

func (g *Generator) SetControllerPKResolver(resolver PrimaryKeyResolver) {
	g.coordinator.ControllerManager.SetPrimaryKeyResolver(resolver)
}

func (g *Generator) GenerateAction(config ActionConfig) error {
	return g.coordinator.ActionManager.GenerateAction(config)
}

func (g *Generator) GetModulePath() string {
	return g.coordinator.projectManager.GetModulePath()
}

func (g *Generator) UpdateModel(resourceName string) (*UpdateModelResult, error) {
	return g.coordinator.ModelManager.UpdateModel(resourceName)
}

func (g *Generator) ApplyModelUpdate(result *UpdateModelResult) error {
	return g.coordinator.ModelManager.ApplyModelUpdate(result)
}

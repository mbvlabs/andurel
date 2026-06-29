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

func (g *Generator) GenerateController(resourceName, tableName string, withViews bool, inertia string) error {
	return g.coordinator.GenerateController(resourceName, tableName, withViews, inertia)
}

func (g *Generator) GenerateControllerWithActions(resourceName, tableName string, withViews bool, actions []string, inertia string) error {
	return g.coordinator.GenerateControllerWithActions(resourceName, tableName, withViews, actions, inertia)
}

func (g *Generator) GenerateControllerWithActionsForModel(resourceName, modelName, tableName string, withViews bool, actions []string, inertia string) error {
	return g.coordinator.GenerateControllerWithActionsForModel(resourceName, modelName, tableName, withViews, actions, inertia)
}

func (g *Generator) GenerateScaffold(resourceName, tableName string, skipFactory bool, primaryKeyColumn string, inertia string) error {
	return g.coordinator.GenerateScaffold(resourceName, tableName, skipFactory, primaryKeyColumn, inertia)
}

func (g *Generator) GenerateControllerFromModel(resourceName string, withViews bool) error {
	return g.coordinator.GenerateControllerFromModel(resourceName, withViews)
}

func (g *Generator) GenerateView(resourceName, tableName string) error {
	return g.coordinator.ViewManager.GenerateView(resourceName, tableName)
}

func (g *Generator) GenerateViewFromModel(resourceName string, withController bool) error {
	return g.coordinator.ViewManager.GenerateViewFromModel(resourceName, withController)
}

func (g *Generator) SetControllerPKResolver(resolver PrimaryKeyResolver) {
	g.coordinator.ControllerManager.SetPrimaryKeyResolver(resolver)
}

func (g *Generator) GenerateFragment(config FragmentConfig) error {
	return g.coordinator.FragmentManager.GenerateFragment(config)
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

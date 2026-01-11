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
	return g.coordinator.ModelManager.GenerateModel(resourceName, tableNameOverride, skipFactory)
}

func (g *Generator) GenerateController(resourceName, tableName string, withViews bool) error {
	return g.coordinator.GenerateController(resourceName, tableName, withViews)
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

func (g *Generator) RefreshModel(resourceName, tableName string) error {
	return g.coordinator.ModelManager.RefreshModel(resourceName, tableName)
}

func (g *Generator) RefreshQueries(resourceName, tableName string) error {
	return g.coordinator.ModelManager.RefreshQueries(resourceName, tableName)
}

func (g *Generator) GenerateQueriesOnly(resourceName string, tableNameOverride string) error {
	return g.coordinator.ModelManager.GenerateQueriesOnly(resourceName, tableNameOverride)
}

func (g *Generator) RefreshQueriesOnly(resourceName, tableName string, tableNameOverridden bool) error {
	return g.coordinator.ModelManager.RefreshQueriesOnly(resourceName, tableName, tableNameOverridden)
}

func (g *Generator) GetModulePath() string {
	return g.coordinator.projectManager.GetModulePath()
}

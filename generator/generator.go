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

func (g *Generator) GenerateModel(resourceName, tableName string) error {
	return g.coordinator.GenerateModel(resourceName, tableName)
}

func (g *Generator) GenerateController(resourceName, tableName string, withViews bool) error {
	return g.coordinator.GenerateController(resourceName, tableName, withViews)
}

func (g *Generator) GenerateControllerFromModel(resourceName string, withViews bool) error {
	return g.coordinator.GenerateControllerFromModel(resourceName, withViews)
}

func (g *Generator) GenerateView(resourceName, tableName string) error {
	return g.coordinator.GenerateView(resourceName, tableName)
}

func (g *Generator) GenerateViewFromModel(resourceName string, withController bool) error {
	return g.coordinator.GenerateViewFromModel(resourceName, withController)
}

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

func (g *Generator) GenerateController(resourceName, tableName string) error {
	return g.coordinator.GenerateController(resourceName, tableName)
}

func (g *Generator) GenerateView(resourceName, tableName string) error {
	return g.coordinator.GenerateView(resourceName, tableName)
}

package generator

import "fmt"

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

func (g *Generator) GenerateController(resourceName ...string) error {
	if len(resourceName) == 1 {
		return g.coordinator.GenerateControllerFromModel(resourceName[0])
	} else if len(resourceName) == 2 {
		return g.coordinator.GenerateController(resourceName[0], resourceName[1])
	}
	return fmt.Errorf("GenerateController requires 1 or 2 arguments")
}

func (g *Generator) GenerateView(resourceName ...string) error {
	if len(resourceName) == 1 {
		return g.coordinator.GenerateViewFromModel(resourceName[0])
	} else if len(resourceName) == 2 {
		return g.coordinator.GenerateView(resourceName[0], resourceName[1])
	}
	return fmt.Errorf("GenerateView requires 1 or 2 arguments")
}

package blueprint_test

import (
	"testing"

	"github.com/mbvlabs/andurel/layout/blueprint"
)

func TestNew(t *testing.T) {
	bp := blueprint.New()

	if bp == nil {
		t.Fatal("expected New to return non-nil blueprint")
	}

	if bp.Controllers.Imports == nil {
		t.Error("expected Controllers.Imports to be initialized")
	}

	if bp.Routes.Imports == nil {
		t.Error("expected Routes.Imports to be initialized")
	}

	if bp.Models.Imports == nil {
		t.Error("expected Models.Imports to be initialized")
	}
}

func TestControllerSection_SortedDependencies(t *testing.T) {
	cs := blueprint.ControllerSection{
		Dependencies: []blueprint.Dependency{
			{Name: "third", Type: "Type3", Order: 2},
			{Name: "first", Type: "Type1", Order: 0},
			{Name: "second", Type: "Type2", Order: 1},
		},
	}

	sorted := cs.SortedDependencies()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(sorted))
	}

	if sorted[0].Name != "first" {
		t.Errorf("expected first dependency to be 'first', got '%s'", sorted[0].Name)
	}

	if sorted[1].Name != "second" {
		t.Errorf("expected second dependency to be 'second', got '%s'", sorted[1].Name)
	}

	if sorted[2].Name != "third" {
		t.Errorf("expected third dependency to be 'third', got '%s'", sorted[2].Name)
	}
}

func TestControllerSection_SortedFields(t *testing.T) {
	cs := blueprint.ControllerSection{
		Fields: []blueprint.Field{
			{Name: "Z", Type: "TypeZ", Order: 2},
			{Name: "A", Type: "TypeA", Order: 0},
			{Name: "M", Type: "TypeM", Order: 1},
		},
	}

	sorted := cs.SortedFields()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(sorted))
	}

	if sorted[0].Name != "A" {
		t.Errorf("expected first field to be 'A', got '%s'", sorted[0].Name)
	}
}

func TestControllerSection_SortedConstructors(t *testing.T) {
	cs := blueprint.ControllerSection{
		Constructors: []blueprint.Constructor{
			{VarName: "c", Expression: "newC()", Order: 2},
			{VarName: "a", Expression: "newA()", Order: 0},
			{VarName: "b", Expression: "newB()", Order: 1},
		},
	}

	sorted := cs.SortedConstructors()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 constructors, got %d", len(sorted))
	}

	if sorted[0].VarName != "a" {
		t.Errorf("expected first constructor to be 'a', got '%s'", sorted[0].VarName)
	}
}

func TestRouteSection_SortedRoutes(t *testing.T) {
	rs := blueprint.RouteSection{
		Routes: []blueprint.Route{
			{Name: "third", Path: "/third", Order: 2},
			{Name: "first", Path: "/first", Order: 0},
			{Name: "second", Path: "/second", Order: 1},
		},
	}

	sorted := rs.SortedRoutes()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(sorted))
	}

	if sorted[0].Name != "first" {
		t.Errorf("expected first route to be 'first', got '%s'", sorted[0].Name)
	}
}

func TestModelSection_SortedModels(t *testing.T) {
	ms := blueprint.ModelSection{
		Models: []blueprint.Model{
			{Name: "C", Order: 2},
			{Name: "A", Order: 0},
			{Name: "B", Order: 1},
		},
	}

	sorted := ms.SortedModels()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 models, got %d", len(sorted))
	}

	if sorted[0].Name != "A" {
		t.Errorf("expected first model to be 'A', got '%s'", sorted[0].Name)
	}
}

func TestConfigSection_SortedFields(t *testing.T) {
	cs := blueprint.ConfigSection{
		Fields: []blueprint.Field{
			{Name: "Z", Type: "string", Order: 2},
			{Name: "A", Type: "int", Order: 0},
			{Name: "M", Type: "bool", Order: 1},
		},
	}

	sorted := cs.SortedFields()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(sorted))
	}

	if sorted[0].Name != "A" {
		t.Errorf("expected first field to be 'A', got '%s'", sorted[0].Name)
	}
}

func TestConfigSection_SortedEnvVars(t *testing.T) {
	cs := blueprint.ConfigSection{
		EnvVars: []blueprint.EnvVar{
			{Key: "KEY_C", ConfigField: "C", Order: 2},
			{Key: "KEY_A", ConfigField: "A", Order: 0},
			{Key: "KEY_B", ConfigField: "B", Order: 1},
		},
	}

	sorted := cs.SortedEnvVars()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(sorted))
	}

	if sorted[0].Key != "KEY_A" {
		t.Errorf("expected first env var to be 'KEY_A', got '%s'", sorted[0].Key)
	}
}

func TestMigrationSection_SortedMigrations(t *testing.T) {
	ms := blueprint.MigrationSection{
		Migrations: []blueprint.Migration{
			{Name: "003_third", Order: 2},
			{Name: "001_first", Order: 0},
			{Name: "002_second", Order: 1},
		},
	}

	sorted := ms.SortedMigrations()

	if len(sorted) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(sorted))
	}

	if sorted[0].Name != "001_first" {
		t.Errorf("expected first migration to be '001_first', got '%s'", sorted[0].Name)
	}
}

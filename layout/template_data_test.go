package layout

import "testing"

func TestTemplateDataBlueprintLazyInit(t *testing.T) {
	td := &TemplateData{}

	if td.blueprint != nil {
		t.Fatalf("expected blueprint to be lazily initialised")
	}

	bp := td.Blueprint()
	if bp == nil {
		t.Fatalf("expected Blueprint to return a non-nil value")
	}

	if td.blueprint != bp {
		t.Fatalf("expected Blueprint to cache the instance")
	}
}

func TestTemplateDataSetBlueprint(t *testing.T) {
	td := &TemplateData{}
	custom := td.Blueprint()

	td.SetBlueprint(custom)

	if td.Blueprint() != custom {
		t.Fatalf("expected SetBlueprint to override the cached blueprint")
	}
}

func TestTemplateDataBuilderUsesBlueprint(t *testing.T) {
	td := &TemplateData{}
	builder := td.Builder()
	if builder == nil {
		t.Fatalf("expected Builder to return a non-nil adapter")
	}

	builder.AddImport("example.com/foo")
	builder.AddControllerDependency("foo", "foo.Service")

	deps := td.Blueprint().Controllers.SortedDependencies()
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].Name != "foo" {
		t.Fatalf("expected dependency name to be foo, got %s", deps[0].Name)
	}
}

func TestTemplateDataGetters(t *testing.T) {
	td := &TemplateData{Database: "sqlite", ModuleName: "acme/app"}

	if got := td.DatabaseDialect(); got != "sqlite" {
		t.Fatalf("unexpected database dialect %q", got)
	}

	if got := td.GetModuleName(); got != "acme/app" {
		t.Fatalf("unexpected module name %q", got)
	}

	var nilData *TemplateData
	if got := nilData.DatabaseDialect(); got != "" {
		t.Fatalf("expected empty dialect for nil, got %q", got)
	}
	if got := nilData.GetModuleName(); got != "" {
		t.Fatalf("expected empty module name for nil, got %q", got)
	}
}

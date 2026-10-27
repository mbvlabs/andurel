package layout

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/blueprint"
)

func TestTemplateDataAccessors(t *testing.T) {
	data := &TemplateData{
		ModuleName: "github.com/example/app",
		Inertia:    "vue",
	}

	if got := data.DatabaseDialect(); got != "postgresql" {
		t.Fatalf("expected postgresql dialect, got %q", got)
	}
	if got := data.GetModuleName(); got != "github.com/example/app" {
		t.Fatalf("expected module name, got %q", got)
	}
	if got := data.GetInertia(); got != "vue" {
		t.Fatalf("expected inertia adapter, got %q", got)
	}

	var nilData *TemplateData
	if got := nilData.GetModuleName(); got != "" {
		t.Fatalf("expected empty module name for nil receiver, got %q", got)
	}
	if got := nilData.GetInertia(); got != "" {
		t.Fatalf("expected empty inertia adapter for nil receiver, got %q", got)
	}
	if bp := nilData.Blueprint(); bp != nil {
		t.Fatalf("expected nil blueprint for nil receiver, got %+v", bp)
	}
	if builder := nilData.Builder(); builder != nil {
		t.Fatalf("expected nil builder for nil receiver, got %+v", builder)
	}
}

func TestTemplateDataSiteNameAndSlug(t *testing.T) {
	for _, tc := range []struct {
		project  string
		siteName string
		slug     string
	}{
		{"andurel-site", "Andurel Site", "andurel-site"},
		{"testapp", "Testapp", "testapp"},
		{"My App 2", "My App 2", "my-app-2"},
	} {
		data := &TemplateData{ProjectName: tc.project}
		if got := data.SiteName(); got != tc.siteName {
			t.Errorf("SiteName(%q) = %q, want %q", tc.project, got, tc.siteName)
		}
		if got := data.Slug(); got != tc.slug {
			t.Errorf("Slug(%q) = %q, want %q", tc.project, got, tc.slug)
		}
	}

	if got := (&TemplateData{Inertia: "react"}).InertiaEntryPoint(); got != "resources/js/app.tsx" {
		t.Fatalf("react entry point = %q", got)
	}
	if got := (&TemplateData{Inertia: "vue"}).InertiaEntryPoint(); got != "resources/js/app.ts" {
		t.Fatalf("vue entry point = %q", got)
	}

	var nilData *TemplateData
	if got := nilData.SiteName(); got != "" {
		t.Fatalf("expected empty site name for nil receiver, got %q", got)
	}
	if got := nilData.Slug(); got != "" {
		t.Fatalf("expected empty slug for nil receiver, got %q", got)
	}
	if got := nilData.InertiaEntryPoint(); got != "resources/js/app.ts" {
		t.Fatalf("expected default entry point for nil receiver, got %q", got)
	}
}

func TestTemplateDataBlueprintLifecycle(t *testing.T) {
	data := &TemplateData{}

	first := data.Blueprint()
	if first == nil {
		t.Fatal("expected Blueprint to lazily initialize")
	}
	if second := data.Blueprint(); second != first {
		t.Fatal("expected Blueprint to return the same instance")
	}

	replacement := blueprint.New()
	data.SetBlueprint(replacement)
	if got := data.Blueprint(); got != replacement {
		t.Fatal("expected SetBlueprint replacement")
	}

	builder := data.Builder()
	if builder == nil || builder.Blueprint() != replacement {
		t.Fatalf("expected builder for replacement blueprint, got %+v", builder)
	}
}

func TestSupportedAdaptersAndRuntimes(t *testing.T) {
	for _, adapter := range []string{"vue", "react", "svelte"} {
		if !IsSupportedInertiaAdapter(adapter) {
			t.Fatalf("expected adapter %q to be supported", adapter)
		}
	}
	for _, adapter := range []string{"", "angular", "Vue"} {
		if IsSupportedInertiaAdapter(adapter) {
			t.Fatalf("expected adapter %q to be unsupported", adapter)
		}
	}

	for _, runtime := range []string{"npm", "pnpm", "bun"} {
		if !IsSupportedJavaScriptRuntime(runtime) {
			t.Fatalf("expected runtime %q to be supported", runtime)
		}
	}
	for _, runtime := range []string{"", "deno", "NPM", "yarn"} {
		if IsSupportedJavaScriptRuntime(runtime) {
			t.Fatalf("expected runtime %q to be unsupported", runtime)
		}
	}
}

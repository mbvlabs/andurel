package cli

import (
	"slices"
	"testing"
)

func TestGenerateModelCatalogNextDistinguishesCustom(t *testing.T) {
	t.Parallel()

	spec, ok := commandMetaSpecs["andurel generate model"]
	if !ok {
		t.Fatal("missing generate model command metadata")
	}

	want := []string{
		"Table models: andurel sync model NAME",
		"Table models: andurel sync factory NAME --check",
		"Table models: andurel inspect models --json",
		"Custom models (--custom): andurel sync queries",
		"Custom models (--custom): andurel doctor --json",
	}
	if !slices.Equal(spec.Next, want) {
		t.Fatalf("generate model next = %#v, want %#v", spec.Next, want)
	}
}

func TestGenerateModelNextBreadcrumbsBranchOnCustom(t *testing.T) {
	t.Parallel()

	table := generateModelNextBreadcrumbs(false)
	if len(table) != 1 || table[0].Command != "andurel doctor" {
		t.Fatalf("table model breadcrumbs = %#v", table)
	}

	custom := generateModelNextBreadcrumbs(true)
	if len(custom) != 2 ||
		custom[0].Command != "andurel sync queries" ||
		custom[1].Command != "andurel doctor --json" {
		t.Fatalf("custom model breadcrumbs = %#v", custom)
	}
}

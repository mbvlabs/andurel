package layout

import "testing"

func TestTemplateDataAddSlotSnippet(t *testing.T) {
	var nilData *TemplateData
	if err := nilData.AddSlotSnippet("controllers:imports", "fmt"); err == nil {
		t.Fatalf("expected error when adding snippet to nil template data")
	}

	td := &TemplateData{}
	if err := td.AddSlotSnippet("", "fmt"); err == nil {
		t.Fatalf("expected error for empty slot name")
	}

	if err := td.AddSlotSnippet("controllers:imports", "fmt"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}
	if err := td.AddSlotSnippet("controllers:imports", "log"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}

	snippets := td.Slot("controllers:imports")
	if len(snippets) != 2 {
		t.Fatalf("unexpected snippet count: got %d want 2", len(snippets))
	}

	snippets[0] = "strings"
	if td.slotSnippets["controllers:imports"][0] != "fmt" {
		t.Fatalf("expected Slot to return a copy of the slice")
	}

	joined := td.SlotJoined("routes:build", ",")
	if joined != "" {
		t.Fatalf("expected empty join for unknown slot, got %q", joined)
	}

	if err := td.AddSlotSnippet("routes:build", "first"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}
	if err := td.AddSlotSnippet("routes:build", "second"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}

	joined = td.SlotJoined("routes:build", ",")
	if joined != "first,second" {
		t.Fatalf("unexpected joined value: got %q", joined)
	}
}

func TestTemplateDataSlotNames(t *testing.T) {
	td := &TemplateData{}

	names := td.SlotNames()
	if len(names) != 0 {
		t.Fatalf("expected no slot names for empty template data, got %v", names)
	}

	if err := td.AddSlotSnippet("controllers:imports", "foo"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}
	if err := td.AddSlotSnippet("cmd/app:imports", "bar"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}

	names = td.SlotNames()
	expected := []string{"cmd/app:imports", "controllers:imports"}
	if len(names) != len(expected) {
		t.Fatalf("unexpected slot name count: got %d want %d", len(names), len(expected))
	}

	for i, want := range expected {
		if names[i] != want {
			t.Fatalf("slot name %d mismatch: got %q want %q", i, names[i], want)
		}
	}
}

func TestTemplateDataStructuredSlots(t *testing.T) {
	var nilData *TemplateData
	if err := nilData.AddSlotData("models:data", 42); err == nil {
		t.Fatalf("expected error when adding slot data to nil template data")
	}

	td := &TemplateData{}
	if err := td.AddSlotData("", 42); err == nil {
		t.Fatalf("expected error for empty slot name")
	}

	if err := td.AddSlotData("models:data", 42); err != nil {
		t.Fatalf("unexpected error adding slot data: %v", err)
	}

	values := td.SlotData("models:data")
	if len(values) != 1 {
		t.Fatalf("unexpected slot data length: got %d want 1", len(values))
	}

	if v, ok := values[0].(int); !ok || v != 42 {
		t.Fatalf("unexpected slot data payload: %#v", values[0])
	}

	values[0] = 99
	if td.structuredSlots["models:data"][0] != 42 {
		t.Fatalf("expected SlotData to return a copy of the slice")
	}

	if td.HasSlot("models:data") {
		t.Fatalf("expected HasSlot to remain false when only structured data exists")
	}

	if !td.HasSlotData("models:data") {
		t.Fatalf("expected HasSlotData to be true for models:data")
	}
}

func TestTemplateDataHasSlotEmpty(t *testing.T) {
	td := &TemplateData{}

	if td.HasSlot("routes:build") {
		t.Fatalf("expected HasSlot to be false when slot is empty")
	}
	if td.HasSlotData("routes:data") {
		t.Fatalf("expected HasSlotData to be false when slot data is empty")
	}

	if err := td.AddSlotSnippet("routes:build", "entry"); err != nil {
		t.Fatalf("unexpected error adding snippet: %v", err)
	}
	if !td.HasSlot("routes:build") {
		t.Fatalf("expected HasSlot to be true after adding snippet")
	}

	if err := td.AddSlotData("routes:data", "value"); err != nil {
		t.Fatalf("unexpected error adding slot data: %v", err)
	}
	if !td.HasSlotData("routes:data") {
		t.Fatalf("expected HasSlotData to be true after adding data")
	}
}

func TestTemplateDataDatabaseDialect(t *testing.T) {
	td := &TemplateData{Database: "sqlite"}
	if got := td.DatabaseDialect(); got != "sqlite" {
		t.Fatalf("unexpected dialect: %q", got)
	}

	var nilData *TemplateData
	if got := nilData.DatabaseDialect(); got != "" {
		t.Fatalf("expected empty dialect for nil, got %q", got)
	}
}

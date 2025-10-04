package layout

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout/extensions"
)

// TemplateData carries the values available to base templates and extension
// contributions. Slot names follow a `<scope>:<region>` naming convention where
// `scope` maps to a logical file (e.g. `controllers`) and `region` describes the
// injection point (e.g. `imports`).
type TemplateData struct {
	ProjectName          string
	ModuleName           string
	Database             string
	SessionKey           string
	SessionEncryptionKey string
	TokenSigningKey      string
	PasswordSalt         string

	slotSnippets    map[string][]string
	structuredSlots map[string][]any
}

// AddSlotSnippet registers a text snippet for the provided slot. Callers should
// prefer deterministic ordering when appending multiple snippets to the same
// slot.
func (td *TemplateData) AddSlotSnippet(slot, snippet string) error {
	if td == nil {
		return fmt.Errorf("layout: template data is nil")
	}

	slot = strings.TrimSpace(slot)
	if slot == "" {
		return fmt.Errorf("layout: slot name cannot be empty")
	}

	if td.slotSnippets == nil {
		td.slotSnippets = make(map[string][]string)
	}

	td.slotSnippets[slot] = append(td.slotSnippets[slot], snippet)
	return nil
}

// AddSlotData adds a structured value to the provided slot. This is intended
// for cases where extensions need to coordinate on richer data than raw text.
func (td *TemplateData) AddSlotData(slot string, value any) error {
	if td == nil {
		return fmt.Errorf("layout: template data is nil")
	}

	slot = strings.TrimSpace(slot)
	if slot == "" {
		return fmt.Errorf("layout: slot name cannot be empty")
	}

	if td.structuredSlots == nil {
		td.structuredSlots = make(map[string][]any)
	}

	td.structuredSlots[slot] = append(td.structuredSlots[slot], value)
	return nil
}

// Slot returns a copy of the snippets registered for the provided slot name.
func (td *TemplateData) Slot(slot string) []string {
	if td == nil {
		return nil
	}

	snippets, ok := td.slotSnippets[slot]
	if !ok {
		return nil
	}

	copySnippets := make([]string, len(snippets))
	copy(copySnippets, snippets)
	return copySnippets
}

// SlotJoined joins all snippets for the slot using the provided separator.
func (td *TemplateData) SlotJoined(slot, sep string) string {
	return strings.Join(td.Slot(slot), sep)
}

// SlotData returns a copy of the structured values registered for the slot.
func (td *TemplateData) SlotData(slot string) []any {
	if td == nil {
		return nil
	}

	values, ok := td.structuredSlots[slot]
	if !ok {
		return nil
	}

	copyValues := make([]any, len(values))
	copy(copyValues, values)
	return copyValues
}

// SlotNames returns a sorted list of slot identifiers that contain snippets.
func (td *TemplateData) SlotNames() []string {
	if td == nil || len(td.slotSnippets) == 0 {
		return nil
	}

	names := make([]string, 0, len(td.slotSnippets))
	for name := range td.slotSnippets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasSlot reports whether the provided slot contains any snippets.
func (td *TemplateData) HasSlot(slot string) bool {
	if td == nil {
		return false
	}

	return len(td.slotSnippets[slot]) > 0
}

// HasSlotData reports whether the provided slot contains structured values.
func (td *TemplateData) HasSlotData(slot string) bool {
	if td == nil {
		return false
	}

	return len(td.structuredSlots[slot]) > 0
}

// DatabaseDialect returns the configured database identifier (e.g. sqlite).
func (td *TemplateData) DatabaseDialect() string {
	if td == nil {
		return ""
	}
	return td.Database
}

var _ extensions.TemplateData = (*TemplateData)(nil)

package patterns

import (
	"fmt"
	"strings"
)

// ManifestFile is one file an agent writes or edits to apply a pattern to a
// concrete entity.
type ManifestFile struct {
	Path    string `json:"path"`
	Action  string `json:"action"`
	Summary string `json:"summary"`
}

// Manifest is the ordered file list for building one entity from a pattern.
// It is the read-only replacement for the old `generate model` convention:
// the same deterministic shape, rendered to stdout instead of written.
type Manifest struct {
	Entity  string         `json:"entity"`
	Table   string         `json:"table"`
	Package string         `json:"package"`
	Files   []ManifestFile `json:"files"`
	Next    []string       `json:"next"`
}

// ManifestFor builds the domain manifest for entity (e.g. "User") on table
// (e.g. "users"). An empty table defaults to the lowercased entity name with
// a naive plural.
func ManifestFor(entity, table string) Manifest {
	name := strings.ToLower(strings.TrimSpace(entity))
	if name == "" {
		name = "entity"
	}
	if strings.TrimSpace(table) == "" {
		table = name + "s"
	}

	title := strings.ToUpper(name[:1]) + name[1:]

	return Manifest{
		Entity:  title,
		Table:   table,
		Package: name,
		Files: []ManifestFile{
			{
				Path:    fmt.Sprintf("domains/%s.go", name),
				Action:  "create",
				Summary: fmt.Sprintf("newtype entity %s with Parse constructors, %s receiver, and Transform", title, title+"s"),
			},
			{
				Path:    fmt.Sprintf("domains/%s.sql", name),
				Action:  "create",
				Summary: "colocated narsilc queries owned by this domain",
			},
			{
				Path:    fmt.Sprintf("domains/%s_test.go", name),
				Action:  "create",
				Summary: "Parse unit tests plus domain tests over a real test database",
			},
			{
				Path:    fmt.Sprintf("migrations/<next>_create_%s_table.sql", table),
				Action:  "create",
				Summary: fmt.Sprintf("table definition; author with: andurel generate migration create_%s_table", table),
			},
			{
				Path:    "internal/runtime/runtime.go",
				Action:  "edit",
				Summary: "construct the domain and pass it to services and handlers",
			},
		},
		Next: []string{
			"andurel sync queries",
			"andurel verify",
			"andurel patterns show handler --entity " + title,
			"andurel patterns show test-domain",
		},
	}
}

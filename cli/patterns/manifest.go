package patterns

import (
	"fmt"
	"strings"
	"unicode"
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
	Entity   string         `json:"entity"`
	Receiver string         `json:"receiver"`
	Table    string         `json:"table"`
	Package  string         `json:"package"`
	Files    []ManifestFile `json:"files"`
	Next     []string       `json:"next"`
}

// ManifestFor builds the domain manifest for entity (e.g. "User" or
// "BlogPost") on table (e.g. "users"). An empty table defaults to the
// snake_cased entity name, pluralized.
func ManifestFor(entity, table string) Manifest {
	title := strings.TrimSpace(entity)
	if title == "" {
		title = "Entity"
	}
	title = strings.ToUpper(title[:1]) + title[1:]

	file := snakeCase(title)
	if strings.TrimSpace(table) == "" {
		table = TableFor(title)
	}
	receiver := Pluralize(title)

	return Manifest{
		Entity:   title,
		Receiver: receiver,
		Table:    table,
		Package:  "domains",
		Files: []ManifestFile{
			{
				Path:    fmt.Sprintf("migrations/<next>_create_%s_table.sql", table),
				Action:  "create",
				Summary: fmt.Sprintf("table definition; author with: andurel generate migration create_%s_table", table),
			},
			{
				Path:    fmt.Sprintf("domains/%s.sql", file),
				Action:  "create",
				Summary: "colocated narsilc queries owned by this domain",
			},
			{
				Path:   fmt.Sprintf("domains/%s.go", file),
				Action: "create",
				Summary: fmt.Sprintf(
					"package domains; entity %s with Validate(); %s receiver with New%s(db storage.Connection) and InTx(tx storage.Tx)",
					title, receiver, receiver,
				),
			},
			{
				Path:    fmt.Sprintf("domains/%s_test.go", file),
				Action:  "create",
				Summary: fmt.Sprintf("Validate unit tests plus domain tests over a real test database via domains.New%s(db)", receiver),
			},
			{
				Path:    "internal/runtime/runtime.go",
				Action:  "edit",
				Summary: fmt.Sprintf("construct with domains.New%s(db) and pass it to the services/handlers that use it", receiver),
			},
		},
		Next: []string{
			"andurel sync queries",
			"andurel verify",
			"andurel inspect domains --json",
			"andurel patterns show handler",
			"andurel patterns show test-domain",
		},
	}
}

// TableFor returns the conventional table name, not a verified schema mapping.
// Irregular plurals and custom table names need an explicit override.
func TableFor(entity string) string {
	return Pluralize(snakeCase(entity))
}

// Pluralize returns a naive English plural: consonant+y → ies;
// s, x, z, ch, sh → es; otherwise s.
func Pluralize(word string) string {
	if word == "" {
		return word
	}
	lower := strings.ToLower(word)
	switch {
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !strings.ContainsRune("aeiou", rune(lower[len(lower)-2])):
		return word[:len(word)-1] + "ies"
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "x"), strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"), strings.HasSuffix(lower, "sh"):
		return word + "es"
	default:
		return word + "s"
	}
}

// snakeCase converts a PascalCase identifier to snake_case (BlogPost →
// blog_post).
func snakeCase(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(runes[i-1]) ||
				(i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1]))) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

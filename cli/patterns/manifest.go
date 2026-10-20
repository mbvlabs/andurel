package patterns

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"
	"unicode"

	"github.com/mbvlabs/andurel/v2/cli/internal/columns"
)

// ManifestFile is one file an agent writes or edits to apply a pattern to a
// concrete entity. Content, when set, is the rendered starting point for the
// file; the agent still writes it.
type ManifestFile struct {
	Path     string `json:"path"`
	Action   string `json:"action"`
	Summary  string `json:"summary"`
	Language string `json:"language,omitempty"`
	Content  string `json:"content,omitempty"`
}

// Manifest is the ordered file list for building one entity from a pattern.
// It is the read-only replacement for the old `generate model` convention:
// the same deterministic shape, rendered to stdout instead of written.
//
// Row is the narsilc row type the entity is rendered from
// (domains/internal/queries.<Entity>Row). RowFound reports whether the
// project's models.go declares it; Columns lists its fields when it does.
type Manifest struct {
	Entity   string                `json:"entity"`
	Receiver string                `json:"receiver"`
	Table    string                `json:"table"`
	Package  string                `json:"package"`
	Row      string                `json:"row"`
	RowFound bool                  `json:"row_found"`
	Columns  []columns.StructField `json:"columns,omitempty"`
	Files    []ManifestFile        `json:"files"`
	Next     []string              `json:"next"`
}

// ModelsPath is the narsilc models file, relative to the project root, that
// holds the <Entity>Row types the domain manifest renders from.
const ModelsPath = "domains/internal/queries/models.go"

// Project is the project context a domain manifest renders from. Module is the
// go.mod module path. Models is the content of ModelsPath, or nil when the
// project has none.
type Project struct {
	Module string
	Models []byte
}

// ManifestFor builds the domain manifest for entity (e.g. "User" or
// "BlogPost") on table (e.g. "users") without project context: the file list
// plus a migration skeleton. An empty table defaults to the snake_cased
// entity name, pluralized.
func ManifestFor(entity, table string) Manifest {
	title, table := manifestNames(entity, table)
	return missingRowManifest(title, table)
}

// ManifestForProject builds the domain manifest from project. When models.go
// declares <Entity>Row, the manifest carries the entity, receiver, Validate()
// stub and CRUD .sql rendered from that row. Otherwise it is ManifestFor.
func ManifestForProject(entity, table string, project Project) (Manifest, error) {
	title, table := manifestNames(entity, table)
	if len(project.Models) == 0 {
		return missingRowManifest(title, table), nil
	}

	file, err := parser.ParseFile(token.NewFileSet(), ModelsPath, project.Models, parser.SkipObjectResolution)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", ModelsPath, err)
	}
	fields, ok := columns.StructFields(file, title+"Row")
	if !ok {
		return missingRowManifest(title, table), nil
	}

	return rowManifest(title, table, project.Module, fields, columns.ImportPaths(file))
}

func manifestNames(entity, table string) (string, string) {
	title := strings.TrimSpace(entity)
	if title == "" {
		title = "Entity"
	}
	title = strings.ToUpper(title[:1]) + title[1:]

	table = strings.TrimSpace(table)
	if table == "" {
		table = TableFor(title)
	}
	return title, table
}

func missingRowManifest(title, table string) Manifest {
	file := snakeCase(title)
	receiver := Pluralize(title)

	return Manifest{
		Entity:   title,
		Receiver: receiver,
		Table:    table,
		Package:  "domains",
		Row:      title + "Row",
		Files: []ManifestFile{
			{
				Path:     fmt.Sprintf("migrations/<next>_create_%s_table.sql", table),
				Action:   "create",
				Summary:  fmt.Sprintf("table definition; author with: andurel generate migration create_%s_table, then replace its body with this skeleton", table),
				Language: "sql",
				Content:  migrationSkeleton(title, table),
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
			"andurel db migrate up",
			"andurel sync queries",
			fmt.Sprintf("andurel patterns show domain --entity %s --table %s", title, table),
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

package patterns

import (
	"fmt"
	"go/format"
	"slices"
	"strings"
	"unicode"

	"github.com/mbvlabs/andurel/v2/cli/internal/columns"
)

// Import paths the rendered domain file always uses.
const (
	storageImport    = "github.com/mbvlabs/andurel/pkg/storage"
	validationImport = "github.com/mbvlabs/andurel/pkg/validation"
)

// rowManifest renders the domain files for an entity whose narsilc row
// exists. The table already exists (a query returns its rows), so the
// manifest has no migration entry.
func rowManifest(
	title, table, module string,
	fields []columns.StructField,
	imports map[string]string,
) (Manifest, error) {
	file := snakeCase(title)
	receiver := Pluralize(title)
	if module == "" {
		module = "app"
	}

	goSource, err := renderDomainGo(title, table, module, fields, imports)
	if err != nil {
		return Manifest{}, err
	}

	return Manifest{
		Entity:   title,
		Receiver: receiver,
		Table:    table,
		Package:  "domains",
		Row:      title + "Row",
		RowFound: true,
		Columns:  fields,
		Files: []ManifestFile{
			{
				Path:   fmt.Sprintf("domains/%s.sql", file),
				Action: "create",
				Summary: fmt.Sprintf(
					"colocated narsilc CRUD queries for %s, columns from queries.%sRow; keep the queries the receiver calls",
					table, title,
				),
				Language: "sql",
				Content:  renderDomainSQL(title, table, fields),
			},
			{
				Path:   fmt.Sprintf("domains/%s.go", file),
				Action: "create",
				Summary: fmt.Sprintf(
					"package domains; entity %s mirrors queries.%sRow with Validate(); %s receiver with New%s(db storage.Connection) and InTx(tx storage.Tx); add the methods that call the queries",
					title, title, receiver, receiver,
				),
				Language: "go",
				Content:  goSource,
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
	}, nil
}

// renderDomainGo renders the receiver, entity and Validate() stub in the shape
// of the scaffold's domains/user.go. Entity field types are the row's types
// unchanged: the scaffold's User keeps every UserRow type as is, including
// pgtype.Timestamptz for created_at, updated_at and the nullable
// email_validated_at, and pgx scans each column into the type narsilc chose.
func renderDomainGo(
	title, table, module string,
	fields []columns.StructField,
	rowImports map[string]string,
) (string, error) {
	receiver := Pluralize(title)
	recv := receiverName(title)

	var std, external []string
	external = append(external, storageImport, validationImport)
	for _, importPath := range fieldImports(fields, rowImports) {
		if isStdImport(importPath) {
			std = append(std, importPath)
		} else {
			external = appendIfMissing(external, importPath)
		}
	}
	slices.Sort(std)
	slices.Sort(external)

	var b strings.Builder
	b.WriteString("package domains\n\nimport (\n")
	for _, importPath := range std {
		fmt.Fprintf(&b, "\t%q\n", importPath)
	}
	if len(std) > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\t%q\n\n", module+"/domains/internal/queries")
	for _, importPath := range external {
		fmt.Fprintf(&b, "\t%q\n", importPath)
	}
	b.WriteString(")\n\n")

	fmt.Fprintf(&b, "type %s struct {\n\tqueries *queries.Queries\n}\n\n", receiver)
	fmt.Fprintf(&b, "func New%s(db storage.Connection) %s {\n\treturn %s{queries: queries.New(db)}\n}\n\n",
		receiver, receiver, receiver)
	b.WriteString("// InTx binds this receiver's queries to the caller's transaction.\n")
	fmt.Fprintf(&b, "func (%s %s) InTx(tx storage.Tx) %s {\n\treturn %s{queries: queries.New(storage.Unwrap(tx))}\n}\n\n",
		recv, receiver, receiver, receiver)

	if table != TableFor(title) {
		fmt.Fprintf(&b, "// andurel:table %s\n", table)
	}
	fmt.Fprintf(&b, "type %s struct {\n", title)
	for _, field := range fields {
		if field.Column == "" {
			fmt.Fprintf(&b, "\t%s %s\n", field.Name, field.Type)
			continue
		}
		fmt.Fprintf(&b, "\t%s %s `andurel:%q`\n", field.Name, field.Type, field.Column)
	}
	b.WriteString("}\n\n")

	// The scaffold names the builder b; an entity starting with B would shadow
	// its own receiver, so it gets a longer name.
	builder := "b"
	if recv == builder {
		builder = "builder"
	}
	fmt.Fprintf(&b, "func (%s *%s) Validate() error {\n\t%s := validation.NewBuilder()\n", recv, title, builder)
	for _, field := range fields {
		if field.Type != "string" {
			continue
		}
		key := lowerCamel(field.Name)
		fmt.Fprintf(&b, "\t// TODO: rules for %s, e.g. %s.Required(%q, %s.%s)\n", field.Name, builder, key, recv, field.Name)
	}
	fmt.Fprintf(&b, "\n\treturn %s.Err()\n}\n", builder)

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return "", fmt.Errorf("format rendered domains/%s.go: %w", snakeCase(title), err)
	}
	return string(formatted), nil
}

// renderDomainSQL renders CRUD queries in the style of the scaffold's
// domains/user.sql. The key is the id column, or the first column when the
// row has none.
func renderDomainSQL(title, table string, fields []columns.StructField) string {
	cols := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Column != "" {
			cols = append(cols, field.Column)
		}
	}
	if len(cols) == 0 {
		return fmt.Sprintf("-- queries.%sRow has no andurel-tagged columns; write the queries by hand.\n", title)
	}
	key := cols[0]
	if slices.Contains(cols, "id") {
		key = "id"
	}
	plural := Pluralize(title)

	var b strings.Builder
	fmt.Fprintf(&b, "-- name: Get%s :one\nSELECT *\nFROM %s\nWHERE %s = $1\nLIMIT 1;\n\n", title, table, key)
	fmt.Fprintf(&b, "-- name: List%s :many\n-- @order %s\nSELECT *\nFROM %s;\n\n", plural, key, table)
	fmt.Fprintf(&b, "-- name: Count%s :one\nSELECT count(*)\nFROM %s;\n\n", plural, table)

	fmt.Fprintf(&b, "-- name: Create%s :one\nINSERT INTO %s (\n", title, table)
	placeholders := make([]string, len(cols))
	for i, col := range cols {
		sep := ","
		if i == len(cols)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "\t%s%s\n", col, sep)
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	fmt.Fprintf(&b, ") VALUES (\n\t%s\n)\nRETURNING *;\n\n", strings.Join(placeholders, ", "))

	var set []string
	for _, col := range cols {
		if col != key && col != "created_at" && col != "updated_at" {
			set = append(set, col)
		}
	}
	if slices.Contains(cols, "updated_at") && key != "updated_at" {
		set = append(set, "updated_at")
	}
	if len(set) > 0 {
		fmt.Fprintf(&b, "-- name: Update%s :one\nUPDATE %s\nSET\n", title, table)
		for i, col := range set {
			sep := ","
			if i == len(set)-1 {
				sep = ""
			}
			fmt.Fprintf(&b, "\t%s = $%d%s\n", col, i+2, sep)
		}
		fmt.Fprintf(&b, "WHERE %s = $1\nRETURNING *;\n\n", key)
	}

	fmt.Fprintf(&b, "-- name: Delete%s :exec\nDELETE FROM %s\nWHERE %s = $1;\n", title, table, key)
	return b.String()
}

// migrationSkeleton is the goose migration for a table that has no narsilc row
// yet, in the shape of the migration pattern.
func migrationSkeleton(title, table string) string {
	return fmt.Sprintf(`-- +goose Up
create table %[1]s (
	id uuid primary key default gen_random_uuid(),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
	-- TODO: add the %[2]s columns, constraints and indexes
);

-- +goose Down
drop table if exists %[1]s;
`, table, title)
}

// fieldImports returns the import paths the field types reference, resolved
// through the row file's imports.
func fieldImports(fields []columns.StructField, rowImports map[string]string) []string {
	var paths []string
	for _, field := range fields {
		for _, name := range qualifiers(field.Type) {
			importPath, ok := rowImports[name]
			if !ok {
				continue
			}
			paths = appendIfMissing(paths, importPath)
		}
	}
	return paths
}

// qualifiers returns the package names a type expression qualifies
// identifiers with (map[string]pgtype.Text → pgtype).
func qualifiers(typeExpr string) []string {
	var names []string
	for i := 0; i < len(typeExpr); i++ {
		if typeExpr[i] != '.' {
			continue
		}
		start := i
		for start > 0 && isIdentRune(rune(typeExpr[start-1])) {
			start--
		}
		if start < i {
			names = appendIfMissing(names, typeExpr[start:i])
		}
	}
	return names
}

func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isStdImport reports whether importPath is in the standard library: its
// first element has no dot. uuid is the standard library's uuid package.
func isStdImport(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

func appendIfMissing(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

// receiverName is the scaffold's receiver variable: the entity's first letter,
// lowercased (User → u).
func receiverName(title string) string {
	return strings.ToLower(title[:1])
}

// lowerCamel converts a Go field name to the validation key style the scaffold
// uses (Email → email, EmailValidatedAt → emailValidatedAt, URLPath → urlPath).
func lowerCamel(name string) string {
	runes := []rune(name)
	upper := 0
	for upper < len(runes) && unicode.IsUpper(runes[upper]) {
		upper++
	}
	switch {
	case upper == 0:
		return name
	case upper == len(runes):
		return strings.ToLower(name)
	case upper > 1:
		upper--
	}
	for i := range upper {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

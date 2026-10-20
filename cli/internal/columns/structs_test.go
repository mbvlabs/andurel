package columns

import (
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"testing"
)

const structsSource = `package queries

import (
	"github.com/jackc/pgx/v5/pgtype"
	"uuid"
	pg "github.com/jackc/pgx/v5"
	_ "embed"
)

type WidgetRow struct {
	ID        uuid.UUID          ` + "`andurel:\"id\"`" + `
	Name, Alias string           ` + "`json:\"name\" andurel:\"name\"`" + `
	Note      *pgtype.Text
	Skipped   string             ` + "`andurel:\"-\"`" + `
}

type NotAStruct int

var _ = pg.ErrNoRows
`

func TestStructFields(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "models.go", structsSource, 0)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := StructFields(file, "WidgetRow")
	if !ok {
		t.Fatal("WidgetRow not found")
	}
	want := []StructField{
		{Name: "ID", Type: "uuid.UUID", Column: "id"},
		{Name: "Name", Type: "string", Column: "name"},
		{Name: "Alias", Type: "string", Column: "name"},
		{Name: "Note", Type: "*pgtype.Text"},
		{Name: "Skipped", Type: "string"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("StructFields = %#v\nwant %#v", got, want)
	}

	if _, ok := StructFields(file, "NotAStruct"); ok {
		t.Error("NotAStruct reported as a struct")
	}
	if _, ok := StructFields(file, "MissingRow"); ok {
		t.Error("MissingRow reported as found")
	}
}

func TestImportPaths(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "models.go", structsSource, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}

	got := ImportPaths(file)
	want := map[string]string{
		"pgtype": "github.com/jackc/pgx/v5/pgtype",
		"uuid":   "uuid",
		"pg":     "github.com/jackc/pgx/v5",
	}
	if !maps.Equal(got, want) {
		t.Fatalf("ImportPaths = %v, want %v", got, want)
	}
}

func TestImportName(t *testing.T) {
	tests := map[string]string{
		"github.com/jackc/pgx/v5":        "pgx",
		"github.com/jackc/pgx/v5/pgtype": "pgtype",
		"uuid":                           "uuid",
		"v2":                             "v2",
	}
	for importPath, want := range tests {
		if got := ImportName(importPath); got != want {
			t.Errorf("ImportName(%q) = %q, want %q", importPath, got, want)
		}
	}
}

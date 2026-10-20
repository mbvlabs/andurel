package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func TestDescribeDomainFileTableSource(t *testing.T) {
	for _, tc := range []struct {
		name, annotation, table, source string
	}{
		{"conventional guess", "", "persons", "inferred"},
		{"irregular override", "// andurel:table people\n", "people", "annotation"},
		{"custom override", "// andurel:table archived_people\n", "archived_people", "annotation"},
		{"empty annotation", "// andurel:table\n", "persons", "inferred"},
		{"unrelated directive", "// andurel:tables people\n", "persons", "inferred"},
		{"tab separator", "// andurel:table\tpeople\n", "people", "annotation"},
		{"no space after slashes", "//andurel:table people\n", "people", "annotation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "person.go", "package domains\n"+tc.annotation+`
type Person struct { ID string `+"`andurel:\"id\"`"+` }
type Persons struct{}
func (*Person) Validate() error { return nil }
`)
			entities := describeDomainFile(filepath.Join(root, "person.go"), map[string]bool{"Persons": true})
			if len(entities) != 1 {
				t.Fatalf("entities = %#v, want one entity", entities)
			}
			entity := entities[0]
			if entity.Table != tc.table || entity.TableSource != tc.source {
				t.Fatalf("table = %q (%s), want %q (%s)", entity.Table, entity.TableSource, tc.table, tc.source)
			}
			if !entity.Validates || !entity.Wired || entity.Receiver != "Persons" || len(entity.Fields) != 1 {
				t.Fatalf("lost domain metadata: %#v", entity)
			}
			data, err := json.Marshal(entity)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"table_source":"`+tc.source+`"`) {
				t.Fatalf("missing table provenance in JSON: %s", data)
			}
		})
	}
}

func TestDescribeDomainFileTableAnnotationIsPerEntity(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "person.go", `package domains

// andurel:table ignored_when_ambiguous

// andurel:table people
type Person struct { ID string `+"`andurel:\"id\"`"+` }

type PersonAddress struct { ID string `+"`andurel:\"id\"`"+` }
`)
	entities := describeDomainFile(filepath.Join(root, "person.go"), nil)
	got := map[string]string{}
	for _, entity := range entities {
		got[entity.Name] = entity.Table + " (" + entity.TableSource + ")"
	}
	want := map[string]string{
		"Person":        "people (annotation)",
		"PersonAddress": "person_addresses (inferred)",
	}
	for name, table := range want {
		if got[name] != table {
			t.Errorf("%s table = %q, want %q", name, got[name], table)
		}
	}
	for _, entity := range entities {
		hasNote := strings.Contains(entity.TableNote, "ignored_when_ambiguous")
		if wantNote := entity.Name == "PersonAddress"; hasNote != wantNote {
			t.Errorf("%s table_note = %q, want note: %v", entity.Name, entity.TableNote, wantNote)
		}
	}
}

const inspectColumnsNarsilcStub = `package narsilc

type Scanner interface {
	Scan(dest ...any) error
}

type Transformer[Row any] interface {
	Transform(Row) error
}

func Scan[T any](s Scanner, columns []string) (T, error) {
	var zero T
	return zero, nil
}

func Read[T, Row any](s Scanner, columns []string) (T, error) {
	return Scan[T](s, columns)
}

func FromRow[T, Row any](row Row) (T, error) {
	var zero T
	_ = row
	return zero, nil
}
`

const inspectColumnsQueries = `package queries

import (
	"context"

	"github.com/mbvlabs/narsilc"
)

type Queries struct{}

type UserRow struct {
	ID       string §andurel:"id"§
	Email    string §andurel:"email"§
	Nickname string §andurel:"nickname"§
}

func (q *Queries) GetUser[T any](ctx context.Context) (T, error) {
	var scanner narsilc.Scanner
	return narsilc.Read[T, UserRow](scanner, []string{"id", "email"})
}
`

const inspectColumnsDomains = `package domains

import (
	"context"

	"example.com/app/domains/internal/queries"
)

type Users struct {
	queries *queries.Queries
}

type User struct {
	ID       string §andurel:"id"§
	Email    string §andurel:"email"§
	Nickname string §andurel:"nickname"§
}

type Orphan struct {
	ID string §andurel:"id"§
}

func (u Users) Find(ctx context.Context) (User, error) {
	return u.queries.GetUser[User](ctx)
}
`

func writeInspectColumnsProject(t *testing.T, userSource string) string {
	t.Helper()

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", `module example.com/app

go 1.27

require github.com/mbvlabs/narsilc v0.0.0

replace github.com/mbvlabs/narsilc => ./stubs/narsilc
`)
	writeTestFile(t, root, "stubs/narsilc/go.mod", "module github.com/mbvlabs/narsilc\n\ngo 1.27\n")
	writeTestFile(t, root, "stubs/narsilc/narsilc.go", inspectSource(inspectColumnsNarsilcStub))
	writeTestFile(
		t,
		root,
		"domains/internal/queries/queries.go",
		inspectSource(inspectColumnsQueries),
	)
	writeTestFile(t, root, "domains/user.go", inspectSource(userSource))
	return root
}

func inspectSource(source string) string {
	return strings.ReplaceAll(source, "§", "`")
}

func runInspectDomains(t *testing.T, root string) ([]projectItem, error) {
	t.Helper()

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var out bytes.Buffer
	cmd := newDomainsCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	if err := cmd.Execute(); err != nil {
		return nil, err
	}

	var envelope struct {
		OK   bool          `json:"ok"`
		Data []projectItem `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK {
		t.Fatalf("inspect domains failed:\n%s", out.String())
	}
	return envelope.Data, nil
}

func inspectEntity(t *testing.T, items []projectItem, name string) domainEntity {
	t.Helper()
	for _, item := range items {
		for _, entity := range item.Entities {
			if entity.Name == name {
				return entity
			}
		}
	}
	t.Fatalf("no %s entity in %#v", name, items)
	return domainEntity{}
}

func TestDomainsCommandReportsEntityColumns(t *testing.T) {
	root := writeInspectColumnsProject(t, inspectColumnsDomains)

	items, err := runInspectDomains(t, root)
	if err != nil {
		t.Fatalf("inspect domains: %v", err)
	}
	user := inspectEntity(t, items, "User")
	if user.ColumnsOK == nil || !*user.ColumnsOK {
		t.Fatalf("User columns_ok = %v, want true", user.ColumnsOK)
	}
	if len(user.Mismatches) != 0 {
		t.Fatalf("User mismatches = %#v, want none", user.Mismatches)
	}
	if len(user.UnpopulatedTags) != 1 ||
		user.UnpopulatedTags[0].Column != "nickname" ||
		user.UnpopulatedTags[0].Field != "Nickname" {
		t.Fatalf("User unpopulated_tags = %#v, want nickname", user.UnpopulatedTags)
	}

	orphan := inspectEntity(t, items, "Orphan")
	if orphan.ColumnsOK != nil {
		t.Fatalf("Orphan columns_ok = %v, want null", *orphan.ColumnsOK)
	}
	if len(orphan.Mismatches) != 0 || len(orphan.UnpopulatedTags) != 0 {
		t.Fatalf("unchecked Orphan must make no claims: %#v", orphan)
	}

	drift := strings.Replace(
		inspectColumnsDomains,
		`Email    string §andurel:"email"§`,
		`Email    string §andurel:"emial"§`,
		1,
	)
	writeTestFile(t, root, "domains/user.go", inspectSource(drift))
	items, err = runInspectDomains(t, root)
	if err != nil {
		t.Fatalf("inspect domains after drift: %v", err)
	}
	user = inspectEntity(t, items, "User")
	if user.ColumnsOK == nil || *user.ColumnsOK {
		t.Fatalf("User columns_ok = %v, want false", user.ColumnsOK)
	}
	if len(user.Mismatches) != 1 || user.Mismatches[0].Kind != "missing-column" {
		t.Fatalf("User mismatches = %#v, want one missing-column", user.Mismatches)
	}
	mismatch := user.Mismatches[0]
	if mismatch.File != "domains/user.go" {
		t.Fatalf("mismatch file = %q, want domains/user.go", mismatch.File)
	}
	if want := inspectSourceLine(t, root, "domains/user.go", "\tEmail "); mismatch.Line != want {
		t.Fatalf("mismatch line = %d, want Email line %d", mismatch.Line, want)
	}
	if mismatch.Column != "email" || mismatch.Field != "Email" {
		t.Fatalf("mismatch = %#v, want email/Email", mismatch)
	}
}

func TestDomainsCommandReportsColumnLoadFailure(t *testing.T) {
	root := writeInspectColumnsProject(t, inspectColumnsDomains)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {\n\tvar value int = \"bad\"\n\t_ = value\n}\n")

	_, err := runInspectDomains(t, root)
	if err == nil {
		t.Fatal("inspect domains should fail when the project cannot be loaded for column checks")
	}
	if !strings.Contains(err.Error(), "inspect domains:") ||
		!strings.Contains(err.Error(), "type-check project") {
		t.Fatalf("error = %v, want an actionable load/type-check failure", err)
	}
}

func inspectSourceLine(t *testing.T, root, rel, needle string) int {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	index := strings.Index(string(content), needle)
	if index < 0 {
		t.Fatalf("%s does not contain %q", rel, needle)
	}
	return strings.Count(string(content[:index]), "\n") + 1
}

package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
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

package golden

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

type inspectDomainEntity struct {
	Name            string              `json:"name"`
	ColumnsOK       *bool               `json:"columns_ok"`
	Mismatches      []inspectColumnNote `json:"mismatches"`
	UnpopulatedTags []inspectColumnNote `json:"unpopulated_tags"`
}

type inspectColumnNote struct {
	Kind   string `json:"kind"`
	Column string `json:"column"`
	Field  string `json:"field"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// tagDrift describes renaming one andurel tag on a scaffold entity field.
type tagDrift struct {
	entity     string // inspect name of the entity, e.g. User
	file       string // domain file relative to the project root
	tag        string // the tag as written, e.g. andurel:"scope"
	drifted    string // the tag after the rename, e.g. andurel:"scopes"
	fieldStart string // the start of the field's declaration line
	field      string // the Go field name, e.g. Scope
	column     string // the row column the entity no longer maps
}

// TestVerifyEntityColumnsDrift is the W5 acceptance path on a real scaffold.
// User and Token both map their rows automatically, so entity-columns checks
// both. The fresh app verifies and inspects clean with columns_ok true for
// each. Renaming a tag so that a query column has no field fails verify with
// entity-columns at the field and shows columns_ok false in inspect. This runs
// for User.Email and Token.Scope, each from a clean scaffold tag.
func TestVerifyEntityColumnsDrift(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)

	parent := t.TempDir()
	goldentest.RunCLIFull(t, parent, "new", "app")
	project := filepath.Join(parent, "app")
	goldentest.SeedProjectTools(t, project)

	goldentest.RunCLI(t, project, "verify", "--no-frontend", "--json")
	before := inspectDomainsJSON(t, project)
	for _, name := range []string{"User", "Token"} {
		entity := findInspectEntity(t, before, name)
		if entity.ColumnsOK == nil || !*entity.ColumnsOK {
			t.Fatalf("fresh %s columns_ok = %v, want true", name, entity.ColumnsOK)
		}
		// Type differences are notes (pgx converts between row and entity types),
		// so only failure kinds would make columns_ok false.
		for _, mismatch := range entity.Mismatches {
			if mismatch.Kind != "type-difference" {
				t.Fatalf("fresh %s mismatches = %#v, want only type-difference notes", name, entity.Mismatches)
			}
		}
	}

	restoreToken := assertTagDriftFails(t, project, tagDrift{
		entity:     "Token",
		file:       "domains/token.go",
		tag:        `andurel:"scope"`,
		drifted:    `andurel:"scopes"`,
		fieldStart: "\tScope     string",
		field:      "Scope",
		column:     "scope",
	})
	restoreToken()

	restored := inspectDomainsJSON(t, project)
	if token := findInspectEntity(t, restored, "Token"); token.ColumnsOK == nil || !*token.ColumnsOK {
		t.Fatalf("restored Token columns_ok = %v, want true", token.ColumnsOK)
	}

	assertTagDriftFails(t, project, tagDrift{
		entity:     "User",
		file:       "domains/user.go",
		tag:        `andurel:"email"`,
		drifted:    `andurel:"emial"`,
		fieldStart: "\tEmail ",
		field:      "Email",
		column:     "email",
	})

	after := inspectDomainsJSON(t, project)
	if token := findInspectEntity(t, after, "Token"); token.ColumnsOK == nil || !*token.ColumnsOK {
		t.Fatalf("Token columns_ok = %v while only User drifts, want true", token.ColumnsOK)
	}
}

// assertTagDriftFails renames the tag in the project, then checks that verify
// exits 9 with entity-columns at the renamed field only, and that inspect
// reports the entity with columns_ok false and one missing-column mismatch at
// that field. It returns a function that restores the original file.
func assertTagDriftFails(t *testing.T, project string, drift tagDrift) (restore func()) {
	t.Helper()

	path := filepath.Join(project, filepath.FromSlash(drift.file))
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", drift.file, err)
	}
	drifted := strings.Replace(string(original), drift.tag, drift.drifted, 1)
	if drifted == string(original) {
		t.Fatalf("%s has no %s tag to rename", drift.file, drift.tag)
	}
	if err := os.WriteFile(path, []byte(drifted), 0o644); err != nil {
		t.Fatalf("write drifted %s: %v", drift.file, err)
	}
	restore = func() {
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatalf("restore %s: %v", drift.file, err)
		}
	}
	fieldLine := sourceLineIn(drifted, drift.fieldStart)
	if fieldLine == 0 {
		t.Fatalf("drifted %s has no %s field", drift.file, drift.field)
	}

	output, exitCode := goldentest.RunCLIExit(t, project, "verify", "--no-frontend", "--json")
	if exitCode != 9 {
		t.Fatalf("verify --json exited %d, want 9\n%s", exitCode, output)
	}

	var verifyEnvelope struct {
		Data struct {
			Steps []struct {
				Name       string `json:"name"`
				Violations []struct {
					Rule string `json:"rule"`
					File string `json:"file"`
					Line int    `json:"line"`
				} `json:"violations"`
			} `json:"steps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &verifyEnvelope); err != nil {
		t.Fatalf("decode verify JSON: %v\n%s", err, output)
	}
	found := false
	for _, step := range verifyEnvelope.Data.Steps {
		for _, violation := range step.Violations {
			if violation.Rule != "entity-columns" {
				continue
			}
			found = true
			if violation.File != drift.file || violation.Line != fieldLine {
				t.Fatalf(
					"entity-columns at %s:%d, want %s:%d\n%s",
					violation.File,
					violation.Line,
					drift.file,
					fieldLine,
					output,
				)
			}
		}
	}
	if !found {
		t.Fatalf("verify did not report entity-columns:\n%s", output)
	}

	entity := findInspectEntity(t, inspectDomainsJSON(t, project), drift.entity)
	if entity.ColumnsOK == nil || *entity.ColumnsOK {
		t.Fatalf("drifted %s columns_ok = %v, want false", drift.entity, entity.ColumnsOK)
	}
	want := inspectColumnNote{Kind: "missing-column", Column: drift.column, Field: drift.field, File: drift.file, Line: fieldLine}
	var failures []inspectColumnNote
	for _, mismatch := range entity.Mismatches {
		if mismatch.Kind != "type-difference" {
			failures = append(failures, mismatch)
		}
	}
	if len(failures) != 1 || failures[0] != want {
		t.Fatalf("drifted %s failures = %#v, want %#v", drift.entity, failures, want)
	}
	return restore
}

func inspectDomainsJSON(t *testing.T, project string) []inspectDomainEntity {
	t.Helper()

	output, exitCode := goldentest.RunCLIExit(t, project, "inspect", "domains", "--json")
	if exitCode != 0 {
		t.Fatalf("inspect domains --json exited %d\n%s", exitCode, output)
	}
	var envelope struct {
		Data []struct {
			Entities []inspectDomainEntity `json:"entities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &envelope); err != nil {
		t.Fatalf("decode inspect domains JSON: %v\n%s", err, output)
	}
	entities := make([]inspectDomainEntity, 0)
	for _, item := range envelope.Data {
		entities = append(entities, item.Entities...)
	}
	return entities
}

func findInspectEntity(t *testing.T, entities []inspectDomainEntity, name string) inspectDomainEntity {
	t.Helper()
	for _, entity := range entities {
		if entity.Name == name {
			return entity
		}
	}
	t.Fatalf("inspect domains has no %s entity: %#v", name, entities)
	return inspectDomainEntity{}
}

func sourceLineIn(source, needle string) int {
	before, _, ok := strings.Cut(source, needle)
	if !ok {
		return 0
	}
	return strings.Count(before, "\n") + 1
}

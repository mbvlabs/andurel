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

// TestVerifyEntityColumnsDrift is the W5 acceptance path on a real scaffold:
// the fresh app verifies and inspects clean, then renaming Token.Scope's tag to
// scopes fails verify with entity-columns at the field and shows columns_ok
// false in inspect. Token maps its row directly, so the check applies to it.
// User implements narsilc.Transformer, so entity-columns does not check it and
// inspect reports columns_ok as null.
func TestVerifyEntityColumnsDrift(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)

	parent := t.TempDir()
	goldentest.RunCLIFull(t, parent, "new", "app")
	project := filepath.Join(parent, "app")
	goldentest.SeedProjectTools(t, project)

	goldentest.RunCLI(t, project, "verify", "--no-frontend", "--json")
	before := inspectDomainsJSON(t, project)
	user := findInspectEntity(t, before, "User")
	if user.ColumnsOK != nil {
		t.Fatalf("fresh User columns_ok = %v, want null because User implements narsilc.Transformer", *user.ColumnsOK)
	}
	token := findInspectEntity(t, before, "Token")
	if token.ColumnsOK == nil || !*token.ColumnsOK {
		t.Fatalf("fresh Token columns_ok = %v, want true", token.ColumnsOK)
	}

	tokenFile := filepath.Join(project, "domains", "token.go")
	content, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("read domains/token.go: %v", err)
	}
	drifted := strings.Replace(string(content), `andurel:"scope"`, `andurel:"scopes"`, 1)
	if drifted == string(content) {
		t.Fatal("domains/token.go has no andurel scope tag to rename")
	}
	if err := os.WriteFile(tokenFile, []byte(drifted), 0o644); err != nil {
		t.Fatalf("write drifted domains/token.go: %v", err)
	}
	fieldLine := sourceLineIn(drifted, "\tScope     string")
	if fieldLine == 0 {
		t.Fatal("drifted domains/token.go has no Scope field")
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
			if violation.File != "domains/token.go" || violation.Line != fieldLine {
				t.Fatalf(
					"entity-columns at %s:%d, want domains/token.go:%d\n%s",
					violation.File,
					violation.Line,
					fieldLine,
					output,
				)
			}
		}
	}
	if !found {
		t.Fatalf("verify did not report entity-columns:\n%s", output)
	}

	after := inspectDomainsJSON(t, project)
	token = findInspectEntity(t, after, "Token")
	if token.ColumnsOK == nil || *token.ColumnsOK {
		t.Fatalf("drifted Token columns_ok = %v, want false", token.ColumnsOK)
	}
	if len(token.Mismatches) != 1 ||
		token.Mismatches[0].Kind != "missing-column" ||
		token.Mismatches[0].Column != "scope" ||
		token.Mismatches[0].Field != "Scope" ||
		token.Mismatches[0].File != "domains/token.go" ||
		token.Mismatches[0].Line != fieldLine {
		t.Fatalf("drifted Token mismatches = %#v, want scope at domains/token.go:%d", token.Mismatches, fieldLine)
	}

	user = findInspectEntity(t, after, "User")
	if user.ColumnsOK != nil {
		t.Fatalf("drifted User columns_ok = %v, want null because User implements narsilc.Transformer", *user.ColumnsOK)
	}
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

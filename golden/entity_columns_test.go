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
// the fresh app verifies and inspects clean, then renaming User.Email's tag to
// emial fails verify with entity-columns at the field and shows columns_ok
// false in inspect.
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
	if user.ColumnsOK == nil || !*user.ColumnsOK {
		t.Fatalf("fresh User columns_ok = %v, want true", user.ColumnsOK)
	}

	userFile := filepath.Join(project, "domains", "user.go")
	content, err := os.ReadFile(userFile)
	if err != nil {
		t.Fatalf("read domains/user.go: %v", err)
	}
	drifted := strings.Replace(string(content), `andurel:"email"`, `andurel:"emial"`, 1)
	if drifted == string(content) {
		t.Fatal("domains/user.go has no andurel email tag to rename")
	}
	if err := os.WriteFile(userFile, []byte(drifted), 0o644); err != nil {
		t.Fatalf("write drifted domains/user.go: %v", err)
	}
	fieldLine := sourceLineIn(drifted, "\tEmail ")
	if fieldLine == 0 {
		t.Fatal("drifted domains/user.go has no Email field")
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
			if violation.File != "domains/user.go" || violation.Line != fieldLine {
				t.Fatalf(
					"entity-columns at %s:%d, want domains/user.go:%d\n%s",
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
	user = findInspectEntity(t, after, "User")
	if user.ColumnsOK == nil || *user.ColumnsOK {
		t.Fatalf("drifted User columns_ok = %v, want false", user.ColumnsOK)
	}
	if len(user.Mismatches) != 1 ||
		user.Mismatches[0].Kind != "missing-column" ||
		user.Mismatches[0].Column != "email" ||
		user.Mismatches[0].Field != "Email" ||
		user.Mismatches[0].File != "domains/user.go" ||
		user.Mismatches[0].Line != fieldLine {
		t.Fatalf("drifted User mismatches = %#v, want email at domains/user.go:%d", user.Mismatches, fieldLine)
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

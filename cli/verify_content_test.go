package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func removeVerifyFile(rootDir, rel string) error {
	return os.Remove(filepath.Join(rootDir, filepath.FromSlash(rel)))
}

const verifyContentManifest = `schemaVersion = 1
include = ["content/**/*.md"]
required = ["content/index.md"]

[[catalog]]
slug = "guide"
file = "content/guide.md"
`

func writeVerifyContentFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, contentManifestName, verifyContentManifest)
	writeTestFile(t, root, "content/index.md", "# Home\n\n[Guide](guide.md)\n")
	writeTestFile(t, root, "content/guide.md", "# Guide\n\n[Home](index.md)\n")
}

func decodeVerifyReport(t *testing.T, result cliTestResult) verifyReport {
	t.Helper()
	envelope := decodeVerifyEnvelope(t, result.stdout)
	return envelope.Data
}

func TestVerifyContentStagePassesDeclaredFixture(t *testing.T) {
	root := newVerifyTestProject(t)
	writeVerifyContentFixture(t, root)

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
	if result.err != nil {
		t.Fatalf("verify --only content: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "content")
	if step.Status != verifyStatusOK || !step.OK {
		t.Fatalf("content step = %#v, want ok", step)
	}
	if step.Rules == nil || len(step.Rules.Executed) != len(contentRuleIDs) {
		t.Fatalf("content rules = %#v, want all content rules executed", step.Rules)
	}
}

func TestVerifyContentNotApplicableWithoutManifest(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
	if result.err != nil {
		t.Fatalf("verify --only content: %v", result.err)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "content")
	if step.Status != verifyStatusSkipped || !step.NotApplicable {
		t.Fatalf("content step = %#v, want skipped not_applicable", step)
	}
	if !strings.Contains(step.Reason, "no content configuration") {
		t.Fatalf("content reason = %q", step.Reason)
	}

	human := runVerifyInProject(t, root, "verify", "--only", "content")
	if human.err != nil {
		t.Fatalf("verify --only content: %v", human.err)
	}
	if !strings.Contains(human.stdout, "content") || !strings.Contains(human.stdout, "SKIP") {
		t.Fatalf("human output should show the skipped content stage:\n%s", human.stdout)
	}
}

func TestVerifyContentDeclaredCheckerFailsTheStage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("checker fixture uses a POSIX shell script")
	}
	root := newVerifyTestProject(t)
	writeTestFile(t, root, contentManifestName, `schemaVersion = 1
include = ["content/**/*.md"]

[[checkers]]
name = "docs-check"
run = ["sh", "-c", "printf 'content/index.md:2: broken catalog entry\\n'; exit 1"]
`)
	writeTestFile(t, root, "content/index.md", "# Home\n")

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "content")
	if len(step.Violations) != 1 {
		t.Fatalf("content violations = %#v, want the checker failure", step.Violations)
	}
	violation := step.Violations[0]
	if violation.Rule != "content-checker" || violation.File != "content/index.md" || violation.Line != 2 {
		t.Fatalf("checker violation = %#v, want content/index.md:2", violation)
	}
}

func TestVerifyContentConfigFailureIsNotHiddenByRuleFilter(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, contentManifestName, `schemaVersion = 2
include = ["content/**/*.md"]
`)

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--rules", "content-link", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "content")
	if len(step.Violations) != 1 || step.Violations[0].Rule != "content-config" {
		t.Fatalf("content violations = %#v, want the content-config failure", step.Violations)
	}
}

func TestVerifyContentEachInvariantFailsIndependently(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(t *testing.T, root string)
		wantRule string
	}{
		{
			name: "missing catalog target",
			mutate: func(t *testing.T, root string) {
				if err := removeVerifyFile(root, "content/guide.md"); err != nil {
					t.Fatal(err)
				}
			},
			wantRule: "content-catalog",
		},
		{
			name: "duplicate slug",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, contentManifestName, verifyContentManifest+`
[[catalog]]
slug = "guide"
file = "content/other.md"
`)
				writeTestFile(t, root, "content/other.md", "# Other\n")
			},
			wantRule: "content-slug",
		},
		{
			name: "missing local link",
			mutate: func(t *testing.T, root string) {
				writeTestFile(t, root, "content/guide.md", "# Guide\n\n[Missing](../missing.md)\n")
			},
			wantRule: "content-link",
		},
		{
			name: "missing required file",
			mutate: func(t *testing.T, root string) {
				if err := removeVerifyFile(root, "content/index.md"); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, root, contentManifestName, strings.Replace(
					verifyContentManifest,
					`required = ["content/index.md"]`,
					`required = ["content/missing.md"]`,
					1,
				))
				writeTestFile(t, root, "content/guide.md", "# Guide\n")
			},
			wantRule: "content-required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newVerifyTestProject(t)
			writeVerifyContentFixture(t, root)
			test.mutate(t, root)

			result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
			assertVerifyExitCode(t, result, output.ExitVerification)
			report, ok := output.Fail(result.err).Data.(verifyReport)
			if !ok {
				t.Fatalf("failure data = %T, want verifyReport", output.Fail(result.err).Data)
			}
			step := findVerifyStep(t, report, "content")
			if step.Status != verifyStatusFail || step.OK {
				t.Fatalf("content step = %#v, want fail", step)
			}
			found := false
			for _, violation := range step.Violations {
				if violation.Rule == test.wantRule && violation.Fix != "" {
					found = true
				}
			}
			if !found {
				t.Fatalf("content violations = %#v, want a %s violation with a fix", step.Violations, test.wantRule)
			}
		})
	}
}

func TestVerifyContentRuleFilter(t *testing.T) {
	root := newVerifyTestProject(t)
	writeVerifyContentFixture(t, root)
	// A broken link but valid slugs: content-link fails, content-slug passes.
	writeTestFile(t, root, "content/guide.md", "# Guide\n\n[Missing](missing.md)\n")

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--rules", "content-slug", "--json")
	if result.err != nil {
		t.Fatalf("verify --rules content-slug: %v", result.err)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "content")
	if step.Rules == nil || len(step.Rules.Selected) != 1 || step.Rules.Selected[0] != "content-slug" {
		t.Fatalf("content rules = %#v, want only content-slug selected", step.Rules)
	}
	if len(step.Rules.Executed) != 1 || step.Rules.Executed[0] != "content-slug" {
		t.Fatalf("content rules = %#v, want content-slug executed", step.Rules)
	}

	failed := runVerifyInProject(t, root, "verify", "--only", "content", "--rules", "content-link", "--json")
	assertVerifyExitCode(t, failed, output.ExitVerification)
	report, ok := output.Fail(failed.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(failed.err).Data)
	}
	step = findVerifyStep(t, report, "content")
	if len(step.Violations) != 1 || step.Violations[0].Rule != "content-link" {
		t.Fatalf("content violations = %#v, want only content-link", step.Violations)
	}
}

func TestVerifyContentJSONIncludesStructuredDiagnostics(t *testing.T) {
	root := newVerifyTestProject(t)
	writeVerifyContentFixture(t, root)
	writeTestFile(t, root, "content/guide.md", "# Guide\n\n[Missing](missing.md)\n")

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "content")
	encoded, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("marshal content step: %v", err)
	}
	for _, want := range []string{`"rule":"content-link"`, `"file":"content/guide.md"`, `"line":3`, `"fix":`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("content JSON is missing %s: %s", want, encoded)
		}
	}
}

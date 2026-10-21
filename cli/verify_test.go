package cli

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
	"github.com/mbvlabs/andurel/v2/cli/output"
)

type verifyTestEnvelope struct {
	OK   bool         `json:"ok"`
	Data verifyReport `json:"data"`
}

func newVerifyTestProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	// Force narsilc to be considered configured so its missing binary is
	// checked in the missing-tool case.
	writeTestFile(t, root, "domains/user.sql", "-- name: FindUser :one\nSELECT 1;\n")
	return root
}

func runVerifyInProject(t *testing.T, rootDir string, args ...string) cliTestResult {
	t.Helper()

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return cliTestResult{
		cmd:    cmd,
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func decodeVerifyEnvelope(t *testing.T, stdout string) verifyTestEnvelope {
	t.Helper()

	var envelope verifyTestEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode verify envelope: %v\n%s", err, stdout)
	}
	return envelope
}

func findVerifyStep(t *testing.T, report verifyReport, name string) verifyStep {
	t.Helper()

	for _, step := range report.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("verify report has no %q step: %#v", name, report.Steps)
	return verifyStep{}
}

func assertVerifyExitCode(t *testing.T, result cliTestResult, want int) {
	t.Helper()

	if got := output.ExitCode(result.err); got != want {
		t.Fatalf("exit code = %d, want %d (err: %v)", got, want, result.err)
	}
}

func TestVerifyReportsOK(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := newVerifyTestProject(t)
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	writeExecutable(t, root, "bin/narsilc", "#!/bin/sh\nexit 0\n")

	result := runVerifyInProject(t, root, "verify", "--json")
	if result.err != nil {
		t.Fatalf(
			"verify --json: %v\nreport: %#v\nstderr:\n%s",
			result.err,
			output.Fail(result.err).Data,
			result.stderr,
		)
	}
	envelope := decodeVerifyEnvelope(t, result.stdout)
	if !envelope.OK || !envelope.Data.OK {
		t.Fatalf("verify should pass: %#v", envelope)
	}
	for _, step := range envelope.Data.Steps {
		if step.Status != verifyStatusOK || !step.OK {
			t.Fatalf("step %q = %#v, want status ok and ok true", step.Name, step)
		}
	}
}

func TestVerifyReportsGeneratedFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := newVerifyTestProject(t)
	// A generation run that writes a file fails the generated step as drift.
	writeExecutable(
		t,
		root,
		"bin/templ",
		"#!/bin/sh\nprintf 'package drift\\n' > generated_drift.go\n",
	)
	writeExecutable(t, root, "bin/narsilc", "#!/bin/sh\nexit 0\n")

	result := runVerifyInProject(t, root, "verify", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)

	failure := output.Fail(result.err)
	if failure.Code != output.CodeVerificationFailed {
		t.Fatalf("error code = %q, want %q", failure.Code, output.CodeVerificationFailed)
	}
	report, ok := failure.Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", failure.Data)
	}
	if report.OK {
		t.Fatalf("report should fail: %#v", report)
	}
	generated := findVerifyStep(t, report, "generated")
	if generated.Status != verifyStatusFail || generated.OK {
		t.Fatalf("generated step = %#v, want status fail and ok false", generated)
	}
	if len(generated.Violations) != 1 || generated.Violations[0].Rule != "generated" {
		t.Fatalf("generated violations = %#v, want one generated rule", generated.Violations)
	}
}

func TestVerifyReportsMissingTools(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)

	failure := output.Fail(result.err)
	report, ok := failure.Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", failure.Data)
	}
	if report.OK {
		t.Fatalf("report should fail: %#v", report)
	}
	generated := findVerifyStep(t, report, "generated")
	if generated.Status != verifyStatusFail || generated.OK {
		t.Fatalf("generated step = %#v, want status fail and ok false", generated)
	}
	if len(generated.Violations) != 2 {
		t.Fatalf("generated violations = %#v, want templ and narsilc", generated.Violations)
	}
	for _, binary := range []string{"templ", "narsilc"} {
		violation, found := findMissingToolViolation(generated.Violations, binary)
		if !found {
			t.Fatalf("missing missing-tool violation for %s: %#v", binary, generated.Violations)
		}
		if violation.Fix != "andurel tool sync" {
			t.Fatalf("%s violation fix = %q, want %q", binary, violation.Fix, "andurel tool sync")
		}
		if !strings.Contains(violation.Message, binary) {
			t.Fatalf("%s violation message = %q, want binary name", binary, violation.Message)
		}
	}
}

func TestVerifySkipGeneratedDoesNotFailRun(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--json")
	if result.err != nil {
		t.Fatalf("verify --skip-generated --json: %v\nstderr:\n%s", result.err, result.stderr)
	}
	envelope := decodeVerifyEnvelope(t, result.stdout)
	if !envelope.OK || !envelope.Data.OK {
		t.Fatalf("explicit skip should pass the run: %#v", envelope)
	}
	generated := findVerifyStep(t, envelope.Data, "generated")
	if generated.Status != verifyStatusSkipped {
		t.Fatalf("generated status = %q, want %q", generated.Status, verifyStatusSkipped)
	}
	if generated.OK {
		t.Fatalf("skipped generated step should keep ok false: %#v", generated)
	}
	if generated.Reason == "" {
		t.Fatalf("skipped generated step should have a reason: %#v", generated)
	}

	human := runVerifyInProject(t, root, "verify", "--skip-generated")
	if human.err != nil {
		t.Fatalf("verify --skip-generated: %v\nstderr:\n%s", human.err, human.stderr)
	}
	if !strings.Contains(human.stdout, "SKIP") {
		t.Fatalf("human output should print SKIP:\n%s", human.stdout)
	}
	if strings.Contains(human.stdout, "FAIL") {
		t.Fatalf("human output should not print FAIL:\n%s", human.stdout)
	}
}

func TestVerifyHelpMentionsMissingTool(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"verify", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify --help: %v", err)
	}
	if !strings.Contains(stdout.String(), "missing-tool") ||
		!strings.Contains(stdout.String(), "andurel tool sync") {
		t.Fatalf("verify --help should mention missing-tool and its fix:\n%s", stdout.String())
	}
}

func TestVerifyHelpListsRuleCatalog(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"verify", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify --help: %v", err)
	}
	for _, rule := range layers.Catalog() {
		if !strings.Contains(stdout.String(), rule.ID) {
			t.Fatalf("verify --help is missing rule %q:\n%s", rule.ID, stdout.String())
		}
		if !strings.Contains(stdout.String(), "fix: "+rule.Fix) {
			t.Fatalf("verify --help is missing the fix for %q:\n%s", rule.ID, stdout.String())
		}
	}
}

// TestVerifyLongHelpRendersRuleCatalog keeps verify's long help and agent
// notes from restating rules by hand: the rule list comes from the catalog, and
// the surrounding prose names no rule ID that is not also a step name.
func TestVerifyLongHelpRendersRuleCatalog(t *testing.T) {
	cmd := newVerifyCommand()
	ruleHelp := verifyRuleHelp()
	if !strings.Contains(cmd.Long, ruleHelp) {
		t.Fatalf("verify long help does not embed the catalog rule list:\n%s", cmd.Long)
	}
	for _, rule := range layers.Catalog() {
		if !strings.Contains(ruleHelp, rule.ID) || !strings.Contains(ruleHelp, "fix: "+rule.Fix) {
			t.Fatalf("catalog rule list is missing %q or its fix:\n%s", rule.ID, ruleHelp)
		}
	}

	steps := map[string]bool{"gofmt": true, "go fix": true, "generated": true}
	meta := commandMetaSpecs["andurel verify"]
	prose := map[string]string{
		"long help":   strings.Replace(cmd.Long, ruleHelp, "", 1),
		"agent notes": cmd.Annotations[agentNotesAnnotation],
		"when to use": strings.Join(meta.WhenToUse, "\n"),
	}
	for _, rule := range layers.Catalog() {
		if steps[rule.ID] {
			continue
		}
		for name, text := range prose {
			if strings.Contains(text, rule.ID) {
				t.Fatalf("verify %s restates rule %q by hand:\n%s", name, rule.ID, text)
			}
		}
	}
}

func TestCommandsJSONIncludesVerifyRules(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"commands", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("commands --json: %v", err)
	}

	var envelope struct {
		Data struct {
			Catalog []struct {
				Path  string        `json:"path"`
				Rules []layers.Rule `json:"rules"`
			} `json:"catalog"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode commands JSON: %v\n%s", err, stdout.String())
	}
	for _, record := range envelope.Data.Catalog {
		if record.Path != "andurel verify" {
			continue
		}
		if len(record.Rules) != len(layers.Catalog()) {
			t.Fatalf("verify rules = %#v, want %#v", record.Rules, layers.Catalog())
		}
		for i, want := range layers.Catalog() {
			if record.Rules[i] != want {
				t.Fatalf("verify rule %d = %#v, want %#v", i, record.Rules[i], want)
			}
		}
		return
	}
	t.Fatal("commands JSON has no andurel verify record")
}

func TestVerifyLayersFailsOnTypeError(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {\n\tvar value int = \"bad\"\n\t_ = value\n}\n")

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)

	failure := output.Fail(result.err)
	report, ok := failure.Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", failure.Data)
	}
	step := findVerifyStep(t, report, "layers")
	if step.Status != verifyStatusFail || step.OK {
		t.Fatalf("layers step = %#v, want fail", step)
	}
	if !strings.Contains(step.Detail, "type-check project") {
		t.Fatalf("layers detail = %q, want type-check failure", step.Detail)
	}
}

// writeVerifyTestPackages adds a package whose TestMain exits 0 before m.Run,
// the way the scaffold's DB-backed packages skip without Docker, and a
// package with a normal passing test.
func writeVerifyTestPackages(t *testing.T, root string) {
	t.Helper()

	writeTestFile(t, root, "dbtests/db.go", "package dbtests\n")
	writeTestFile(t, root, "dbtests/main_test.go", `package dbtests

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Fprintln(os.Stderr, "skipping db tests: start postgres container: docker not found")
	os.Exit(0)
}

func TestNeedsDB(t *testing.T) {}
`)
	writeTestFile(t, root, "plain/plain.go", "package plain\n")
	writeTestFile(t, root, "plain/plain_test.go", "package plain\n\nimport \"testing\"\n\nfunc TestPlain(t *testing.T) {}\n")
}

func TestVerifyWithTestsReportsSkippedPackages(t *testing.T) {
	root := newVerifyTestProject(t)
	writeVerifyTestPackages(t, root)

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests", "--json")
	if result.err != nil {
		t.Fatalf(
			"verify --with-tests --json: %v\nreport: %#v",
			result.err,
			output.Fail(result.err).Data,
		)
	}
	envelope := decodeVerifyEnvelope(t, result.stdout)
	if !envelope.OK || !envelope.Data.OK {
		t.Fatalf("a TestMain skip should not fail the run: %#v", envelope)
	}
	step := findVerifyStep(t, envelope.Data, "go test")
	if step.Status != verifyStatusOK || !step.OK {
		t.Fatalf("go test step = %#v, want status ok", step)
	}
	if len(step.SkippedPackages) != 1 {
		t.Fatalf("skipped packages = %#v, want only dbtests", step.SkippedPackages)
	}
	skipped := step.SkippedPackages[0]
	if skipped.Package != "example.com/app/dbtests" {
		t.Fatalf("skipped package = %q, want example.com/app/dbtests", skipped.Package)
	}
	if skipped.Reason != "skipping db tests: start postgres container: docker not found" {
		t.Fatalf("skipped reason = %q, want the TestMain message", skipped.Reason)
	}

	human := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests")
	if human.err != nil {
		t.Fatalf("verify --with-tests: %v", human.err)
	}
	for _, want := range []string{
		"go test    ok (1 package skipped: no tests ran)",
		"skipped example.com/app/dbtests: skipping db tests",
	} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("human output is missing %q:\n%s", want, human.stdout)
		}
	}
	if strings.Contains(human.stdout, "plain") {
		t.Fatalf("a package whose tests ran should not be listed:\n%s", human.stdout)
	}
}

func TestVerifyWithTestsFailsOnTestFailure(t *testing.T) {
	root := newVerifyTestProject(t)
	writeVerifyTestPackages(t, root)
	writeTestFile(t, root, "broken/broken_test.go", `package broken

import "testing"

func TestBroken(t *testing.T) {
	t.Fatal("expected failure marker")
}
`)

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)

	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "go test")
	if step.Status != verifyStatusFail || step.OK {
		t.Fatalf("go test step = %#v, want fail", step)
	}
	for _, want := range []string{"--- FAIL: TestBroken", "expected failure marker", "FAIL\texample.com/app/broken"} {
		if !strings.Contains(step.Detail, want) {
			t.Fatalf("go test detail is missing %q:\n%s", want, step.Detail)
		}
	}
	if strings.Contains(step.Detail, "TestPlain") {
		t.Fatalf("go test detail should only show failing packages:\n%s", step.Detail)
	}
}

func TestVerifyWithTestsFailsOnBuildError(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "broken/broken_test.go", `package broken

import "testing"

func TestBroken(t *testing.T) {
	var value int = "bad"
	_ = value
}
`)

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)

	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "go test")
	if step.Status != verifyStatusFail {
		t.Fatalf("go test step = %#v, want fail", step)
	}
	if !strings.Contains(step.Detail, "broken_test.go") || !strings.Contains(step.Detail, "[build failed]") {
		t.Fatalf("go test detail should show the build error:\n%s", step.Detail)
	}
}

func findMissingToolViolation(
	violations []verifyViolation,
	binary string,
) (verifyViolation, bool) {
	for _, violation := range violations {
		if violation.Rule == "missing-tool" && strings.Contains(violation.Message, binary) {
			return violation, true
		}
	}
	return verifyViolation{}, false
}

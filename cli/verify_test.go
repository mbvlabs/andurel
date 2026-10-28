package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/emailcompiler"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/spf13/cobra"
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
		if step.Name == "frontend" && step.Status == verifyStatusSkipped {
			continue
		}
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

	steps := map[string]bool{
		"gofmt":     true,
		"go fix":    true,
		"frontend":  true,
		"generated": true,
	}
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

func TestVerifyWithTestsReportsSkippedTests(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "skiptests/skip.go", "package skiptests\n")
	writeTestFile(t, root, "skiptests/skip_test.go", `package skiptests

import "testing"

func TestPasses(t *testing.T) {}

func TestNeedsAssets(t *testing.T) {
	t.Skip("assets/dist is missing; run pnpm build")
}
`)

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
		t.Fatalf("a test-level skip should not fail the run: %#v", envelope)
	}
	step := findVerifyStep(t, envelope.Data, "go test")
	if step.Status != verifyStatusOK || !step.OK {
		t.Fatalf("go test step = %#v, want status ok", step)
	}
	if len(step.SkippedPackages) != 0 {
		t.Fatalf("a package with a passing test is not skipped: %#v", step.SkippedPackages)
	}
	if len(step.SkippedTests) != 1 {
		t.Fatalf("skipped tests = %#v, want only TestNeedsAssets", step.SkippedTests)
	}
	skipped := step.SkippedTests[0]
	if skipped.Package != "example.com/app/skiptests" || skipped.Test != "TestNeedsAssets" {
		t.Fatalf("skipped test = %#v, want skiptests.TestNeedsAssets", skipped)
	}
	if !strings.Contains(skipped.Reason, "assets/dist is missing") {
		t.Fatalf("skipped reason = %q, want the t.Skip message", skipped.Reason)
	}

	human := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests")
	if human.err != nil {
		t.Fatalf("verify --with-tests: %v", human.err)
	}
	for _, want := range []string{
		"go test    ok (1 test skipped)",
		"skipped example.com/app/skiptests: TestNeedsAssets (",
		"assets/dist is missing",
	} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("human output is missing %q:\n%s", want, human.stdout)
		}
	}
	if strings.Contains(human.stdout, "TestPasses") {
		t.Fatalf("a passing test should not be listed:\n%s", human.stdout)
	}
}

func TestVerifyGeneratedWarnsOnUnresolvedFromStructRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := setupPayloadsProject(t, "react")
	writePayloadsHandler(t, root, "handlers/pages.go", unresolvedFromStructHandlerSource)
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	if err := generateInertiaArtifacts(root, "react"); err != nil {
		t.Fatalf("generate inertia artifacts: %v", err)
	}

	step := verifyGenerated(root)
	if step.Status != verifyStatusOK || !step.OK {
		t.Fatalf("unresolved roots must not fail verify: %#v", step)
	}
	if len(step.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want one unresolved FromStruct root", step.Warnings)
	}
	warning := step.Warnings[0]
	for _, want := range []string{
		"payloads.ts: handlers/pages.go:",
		unresolvedFromStructReason,
		"pass a named composite literal or a typed variable",
	} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning is missing %q: %s", want, warning)
		}
	}

	structured, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("marshal go test step: %v", err)
	}
	if !bytes.Contains(structured, []byte(`"warnings"`)) {
		t.Fatalf("JSON step is missing warnings: %s", structured)
	}

	var human bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&human)
	if err := renderVerifyHuman(cmd, verifyReport{OK: true, Steps: []verifyStep{step}}); err != nil {
		t.Fatalf("render verify: %v", err)
	}
	if !strings.Contains(human.String(), "warning: "+warning) {
		t.Fatalf("human output is missing the warning:\n%s", human.String())
	}
}

func TestVerifyHelpMentionsSkippedTests(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"verify", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify --help: %v", err)
	}
	for _, want := range []string{"skipped_tests", "warnings"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("verify --help should mention %q:\n%s", want, stdout.String())
		}
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

func TestVerifyScopesGoChecksToPackages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := newVerifyTestProject(t)
	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	writeExecutable(t, root, "bin/narsilc", "#!/bin/sh\nexit 0\n")
	writeTestFile(t, root, "alpha/alpha.go", "package alpha\n")
	writeTestFile(t, root, "beta/beta.go", "package beta\n\nfunc  Broken() {}\n")

	scoped := runVerifyInProject(t, root, "verify", "./alpha/...", "--json")
	if scoped.err != nil {
		t.Fatalf(
			"scoped verify: %v\nreport: %#v",
			scoped.err,
			output.Fail(scoped.err).Data,
		)
	}
	envelope := decodeVerifyEnvelope(t, scoped.stdout)
	gofmt := findVerifyStep(t, envelope.Data, "gofmt")
	if gofmt.Status != verifyStatusOK {
		t.Fatalf("scoped gofmt = %#v, want ok", gofmt)
	}

	drifted := runVerifyInProject(t, root, "verify", "./beta/...", "--json")
	assertVerifyExitCode(t, drifted, output.ExitVerification)
	report, ok := output.Fail(drifted.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(drifted.err).Data)
	}
	gofmt = findVerifyStep(t, report, "gofmt")
	if gofmt.Status != verifyStatusFail {
		t.Fatalf("scoped gofmt = %#v, want fail", gofmt)
	}
	if len(gofmt.Violations) != 1 || gofmt.Violations[0].File != "beta/beta.go" {
		t.Fatalf("scoped gofmt violations = %#v, want beta/beta.go", gofmt.Violations)
	}
}

func TestVerifyNoFrontendSkipsTypecheck(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)

	result := runVerifyInProject(t, root, "verify", "--no-frontend", "--skip-generated", "--json")
	if result.err != nil {
		t.Fatalf("verify --no-frontend --json: %v\n%s", result.err, result.stderr)
	}
	envelope := decodeVerifyEnvelope(t, result.stdout)
	step := findVerifyStep(t, envelope.Data, "frontend")
	if step.Status != verifyStatusSkipped || step.Reason != "--no-frontend" {
		t.Fatalf("frontend step = %#v, want skipped by --no-frontend", step)
	}
}

func TestVerifyFrontendTypecheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := t.TempDir()
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "pnpm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	step := verifyFrontend(root)
	if step.Status != verifyStatusOK || !step.OK {
		t.Fatalf("frontend step = %#v, want ok", step)
	}
}

func TestVerifyFrontendUsesLockPackageManager(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := t.TempDir()
	lock := layout.NewAndurelLock("test")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:              "app",
		JavaScriptPackageManager: "bun",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "pnpm", "#!/bin/sh\nexit 0\n")
	writeExecutable(t, fakeBin, "bun", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	step := verifyFrontend(root)
	if step.Status != verifyStatusFail {
		t.Fatalf("frontend step = %#v, want the bun failure", step)
	}
}

func TestVerifyFrontendMissingNodeModules(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)

	step := verifyFrontend(root)
	if step.Status != verifyStatusFail || step.OK {
		t.Fatalf("frontend step = %#v, want fail", step)
	}
	if len(step.Violations) != 1 || step.Violations[0].Rule != "frontend" {
		t.Fatalf("frontend violations = %#v, want one frontend rule", step.Violations)
	}
	if !strings.Contains(step.Violations[0].Message, "node_modules") {
		t.Fatalf("frontend message = %q, want node_modules hint", step.Violations[0].Message)
	}
	if step.Violations[0].Fix != "pnpm install" {
		t.Fatalf("frontend fix = %q, want pnpm install", step.Violations[0].Fix)
	}
}

func TestVerifyFrontendWithoutTypecheckIsSkipped(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "package.json", `{"scripts":{"build":"vite build"}}`)

	step := verifyFrontend(root)
	if step.Status != verifyStatusSkipped || step.OK {
		t.Fatalf("frontend step = %#v, want skipped", step)
	}
	if !strings.Contains(step.Reason, "typecheck") {
		t.Fatalf("frontend reason = %q, want missing typecheck script", step.Reason)
	}
}

func TestVerifyEmailGeneratedDriftUsesEmailCompiler(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}

	root := t.TempDir()
	writeTestFile(t, root, "email/welcome.templ", `package email

templ Welcome() {
	<p class="p-4">Hi</p>
}
`)
	writeTestFile(t, root, "css/email.css", "")
	writeExecutable(t, root, "bin/tailwindcli", `#!/bin/sh
output=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "-o" ]; then
		shift
		output="$1"
	fi
	shift
done
printf '%s\n' '.p-4 { padding: 1rem; }' > "$output"
`)

	stale := checkEmailGenerate(root, false)
	if stale.status != statusFail || len(stale.changed) != 1 ||
		stale.changed[0] != "email/welcome_templ.go" {
		t.Fatalf("stale email check = %#v, want email/welcome_templ.go drift", stale)
	}

	if err := emailcompiler.Compile(context.Background(), emailcompiler.Config{
		ProjectRoot:  root,
		TailwindPath: filepath.Join(root, "bin", "tailwindcli"),
	}); err != nil {
		t.Fatalf("compile email: %v", err)
	}
	current := checkEmailGenerate(root, false)
	if current.status != statusPass {
		t.Fatalf("current email check = %#v, want pass", current)
	}
}

func TestGeneratedViolationsNameSourceFiles(t *testing.T) {
	templ := generatedViolations(checkResult{
		name:    "templ generate",
		status:  statusFail,
		message: "generated templates are out of date",
		changed: []string{"handlers/pages_templ.go"},
	})
	if len(templ) != 1 || templ[0].File != "handlers/pages.templ" ||
		templ[0].Fix != "andurel build" {
		t.Fatalf("templ violations = %#v, want handlers/pages.templ", templ)
	}

	email := generatedViolations(checkResult{
		name:    "email generate",
		status:  statusFail,
		message: "generated email renderers are out of date",
		changed: []string{"email/welcome_templ.go"},
	})
	if len(email) != 1 || email[0].File != "email/welcome.templ" ||
		email[0].Fix != "andurel sync email" {
		t.Fatalf("email violations = %#v, want email/welcome.templ", email)
	}
}

func TestEmailCompileCommandJSONReportsFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub tools use a POSIX shell script")
	}
	resetCLITestSeams(t)

	root := t.TempDir()
	findGoModRoot = func() (string, error) { return root, nil }
	if err := layout.NewAndurelLock("test").WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	writeTestFile(t, root, "email/welcome.templ", `package email

templ Welcome() {
	<p class="p-4">Hi</p>
}
`)
	writeTestFile(t, root, "css/email.css", "")
	writeExecutable(t, root, "bin/tailwindcli", `#!/bin/sh
output=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "-o" ]; then
		shift
		output="$1"
	fi
	shift
done
printf '%s\n' '.p-4 { padding: 1rem; }' > "$output"
`)
	syncSingleToolFunc = func(string, string, *layout.Tool, string, string) error { return nil }

	command := newEmailCompileCommand()
	output.RegisterPersistentFlags(command)
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stdout)
	command.SetArgs([]string{"--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("sync email --json: %v\n%s", err, stdout.String())
	}

	var envelope struct {
		OK   bool               `json:"ok"`
		Data emailCompileOutput `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode sync email JSON: %v\n%s", err, stdout.String())
	}
	if !envelope.OK {
		t.Fatalf("sync email JSON = %s", stdout.String())
	}
	if len(envelope.Data.GeneratedFiles) != 1 ||
		envelope.Data.GeneratedFiles[0] != "email/welcome_templ.go" {
		t.Fatalf("generated files = %#v", envelope.Data.GeneratedFiles)
	}
	if len(envelope.Data.ChangedFiles) != 1 ||
		envelope.Data.ChangedFiles[0] != "email/welcome_templ.go" {
		t.Fatalf("changed files = %#v", envelope.Data.ChangedFiles)
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

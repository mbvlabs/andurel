package cli

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

// newGitVerifyProject creates a committed Go project with one intentionally
// unformatted package (zeta) so whole-project and changed scope can be told
// apart.
func newGitVerifyProject(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := newVerifyTestProject(t)
	writeTestFile(t, root, ".gitignore", "bin/\nnode_modules/\n")
	writeTestFile(t, root, "alpha/alpha.go", "package alpha\n")
	writeTestFile(t, root, "zeta/zeta.go", "package zeta\n\nfunc  Broken() {}\n")
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "baseline")
	return root
}

func TestVerifyOnlySelectsStages(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--only", "gofmt", "--json")
	if result.err != nil {
		t.Fatalf("verify --only gofmt: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	gofmt := findVerifyStep(t, report, "gofmt")
	if gofmt.Status != verifyStatusOK || !gofmt.OK {
		t.Fatalf("gofmt step = %#v, want ok", gofmt)
	}
	vet := findVerifyStep(t, report, "go vet")
	if vet.Status != verifyStatusSkipped || !strings.Contains(vet.Reason, "not selected") {
		t.Fatalf("go vet step = %#v, want skipped not selected", vet)
	}
	frontend := findVerifyStep(t, report, "frontend")
	if frontend.Status != verifyStatusSkipped {
		t.Fatalf("frontend step = %#v, want skipped", frontend)
	}
	if report.Scope.Mode != verifyScopeAll {
		t.Fatalf("scope = %#v, want all", report.Scope)
	}
	if len(report.Scope.Stages) != 1 || report.Scope.Stages[0] != verifyStageGofmt {
		t.Fatalf("scope stages = %#v, want [gofmt]", report.Scope.Stages)
	}
}

func TestVerifyUnknownStageAndRuleAreUsageErrors(t *testing.T) {
	root := newVerifyTestProject(t)
	for _, args := range [][]string{
		{"verify", "--only", "definitely-not-a-stage", "--json"},
		{"verify", "--rules", "definitely-not-a-rule", "--json"},
	} {
		result := runVerifyInProject(t, root, args...)
		assertVerifyExitCode(t, result, output.ExitUsage)
		failure := output.Fail(result.err)
		if !strings.Contains(failure.Error+failure.Hint, "unknown verify") {
			t.Fatalf("%v error = %q / %q, want an unknown-name usage error", args, failure.Error, failure.Hint)
		}
	}
}

func TestVerifyRulesSelectOwningStageAndReportExecuted(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--rules", "db-handle", "--json")
	if result.err != nil {
		t.Fatalf("verify --rules db-handle: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	layers := findVerifyStep(t, report, "layers")
	if layers.Status != verifyStatusOK || layers.Rules == nil {
		t.Fatalf("layers step = %#v, want ok with rules", layers)
	}
	if len(layers.Rules.Selected) != 1 || layers.Rules.Selected[0] != "db-handle" {
		t.Fatalf("layers rules = %#v, want db-handle selected", layers.Rules)
	}
	if len(layers.Rules.Executed) != 1 || layers.Rules.Executed[0] != "db-handle" {
		t.Fatalf("layers rules = %#v, want db-handle executed", layers.Rules)
	}
	if len(report.Scope.Stages) != 1 || report.Scope.Stages[0] != verifyStageLayers {
		t.Fatalf("scope stages = %#v, want [layers]", report.Scope.Stages)
	}
}

func TestVerifyChangedScopesToChangedGoFiles(t *testing.T) {
	root := newGitVerifyProject(t)

	whole := runVerifyInProject(t, root, "verify", "--only", "gofmt", "--json")
	assertVerifyExitCode(t, whole, output.ExitVerification)
	wholeReport, ok := output.Fail(whole.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(whole.err).Data)
	}
	wholeGofmt := findVerifyStep(t, wholeReport, "gofmt")
	if len(wholeGofmt.Violations) != 1 || wholeGofmt.Violations[0].File != "zeta/zeta.go" {
		t.Fatalf("whole gofmt violations = %#v, want zeta/zeta.go", wholeGofmt.Violations)
	}

	writeTestFile(t, root, "alpha/alpha.go", "package alpha\n\nfunc  Alpha() {}\n")
	changed := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt", "--json")
	assertVerifyExitCode(t, changed, output.ExitVerification)
	report, ok := output.Fail(changed.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(changed.err).Data)
	}
	gofmt := findVerifyStep(t, report, "gofmt")
	if len(gofmt.Violations) != 1 || gofmt.Violations[0].File != "alpha/alpha.go" {
		t.Fatalf("changed gofmt violations = %#v, want only alpha/alpha.go", gofmt.Violations)
	}
	if report.Scope.Mode != verifyScopeChanged || report.Scope.Base != "HEAD" {
		t.Fatalf("scope = %#v, want changed with base HEAD", report.Scope)
	}
	if len(report.Scope.ChangedFiles) != 1 || report.Scope.ChangedFiles[0] != "alpha/alpha.go" {
		t.Fatalf("changed files = %#v", report.Scope.ChangedFiles)
	}
	if len(report.Scope.GoPackages) == 0 {
		t.Fatalf("scope Go packages are empty: %#v", report.Scope)
	}
}

func TestVerifyChangedIncludesUntrackedFiles(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, "untracked/untracked.go", "package untracked\n\nfunc  New() {}\n")

	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	gofmt := findVerifyStep(t, report, "gofmt")
	if len(gofmt.Violations) != 1 || gofmt.Violations[0].File != "untracked/untracked.go" {
		t.Fatalf("gofmt violations = %#v, want the untracked file", gofmt.Violations)
	}
}

func TestVerifyChangedDeletedGoFileScopesToOwningPackage(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, "alpha/helper.go", "package alpha\n")
	runGit(t, root, "add", "alpha/helper.go")
	runGit(t, root, "commit", "-m", "add helper")
	if err := removeVerifyFile(root, "alpha/helper.go"); err != nil {
		t.Fatalf("delete helper: %v", err)
	}

	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt,layers", "--json")
	if result.err != nil {
		t.Fatalf("verify --changed after deletion: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	gofmt := findVerifyStep(t, report, "gofmt")
	if gofmt.Status != verifyStatusSkipped || !strings.Contains(gofmt.Reason, "deletions and renames") {
		t.Fatalf("gofmt step = %#v, want skipped for a deletion-only change", gofmt)
	}
	layers := findVerifyStep(t, report, "layers")
	if layers.Status != verifyStatusOK {
		t.Fatalf("layers step = %#v, want the owning package checked", layers)
	}
	if len(report.Scope.GoPackages) == 0 {
		t.Fatalf("scope Go packages are empty for a deleted Go file: %#v", report.Scope)
	}
}

func TestVerifyChangedEmptySetIsExplicitNoop(t *testing.T) {
	root := newGitVerifyProject(t)

	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt", "--json")
	if result.err != nil {
		t.Fatalf("verify --changed: %v", result.err)
	}
	report := decodeVerifyReport(t, result)
	if !report.Scope.Noop {
		t.Fatalf("scope = %#v, want noop", report.Scope)
	}
	gofmt := findVerifyStep(t, report, "gofmt")
	if gofmt.Status != verifyStatusSkipped || !strings.Contains(gofmt.Reason, "no changed files") {
		t.Fatalf("gofmt step = %#v, want explicit no-op skip", gofmt)
	}
	if !strings.Contains(verifySummary(report), "No changes in scope") {
		t.Fatalf("summary = %q, want the explicit no-op message", verifySummary(report))
	}

	human := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt")
	if human.err != nil {
		t.Fatalf("verify --changed: %v", human.err)
	}
	if !strings.Contains(human.stdout, "No changes in scope") {
		t.Fatalf("human output is missing the no-op note:\n%s", human.stdout)
	}
}

func TestVerifyChangedUsageErrors(t *testing.T) {
	t.Run("not a git repository", func(t *testing.T) {
		root := newVerifyTestProject(t)
		result := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt", "--json")
		assertVerifyExitCode(t, result, output.ExitUsage)
		if !strings.Contains(output.Fail(result.err).Error, "requires a Git repository") {
			t.Fatalf("error = %v", result.err)
		}
	})
	t.Run("invalid base", func(t *testing.T) {
		root := newGitVerifyProject(t)
		result := runVerifyInProject(
			t, root, "verify", "--changed", "--base", "definitely-not-a-ref", "--only", "gofmt", "--json",
		)
		assertVerifyExitCode(t, result, output.ExitUsage)
		if !strings.Contains(output.Fail(result.err).Error, "not a valid Git revision") {
			t.Fatalf("error = %v", result.err)
		}
	})
	t.Run("patterns conflict", func(t *testing.T) {
		root := newGitVerifyProject(t)
		result := runVerifyInProject(t, root, "verify", "--changed", "./alpha/...", "--json")
		assertVerifyExitCode(t, result, output.ExitUsage)
		if !strings.Contains(output.Fail(result.err).Error, "cannot be combined") {
			t.Fatalf("error = %v", result.err)
		}
	})
	t.Run("base without changed", func(t *testing.T) {
		root := newGitVerifyProject(t)
		result := runVerifyInProject(t, root, "verify", "--base", "HEAD~1", "--only", "gofmt", "--json")
		assertVerifyExitCode(t, result, output.ExitUsage)
		if !strings.Contains(output.Fail(result.err).Error, "requires --changed") {
			t.Fatalf("error = %v", result.err)
		}
	})
}

func TestVerifyChangedWidensOnGlobalGoFiles(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.21\n")

	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "gofmt", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	gofmt := findVerifyStep(t, report, "gofmt")
	if len(gofmt.Violations) != 1 || gofmt.Violations[0].File != "zeta/zeta.go" {
		t.Fatalf("gofmt violations = %#v, want the whole-module zeta file", gofmt.Violations)
	}
	widened := false
	for _, widening := range report.Scope.Widened {
		if strings.HasPrefix(widening.Step, "Go checks") {
			widened = true
		}
	}
	if !widened {
		t.Fatalf("scope.widened = %#v, want the global Go change", report.Scope.Widened)
	}
}

func TestVerifyChangedDeletionAndRenameAffectContent(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, contentManifestName, verifyContentManifest)
	writeTestFile(t, root, "content/index.md", "# Home\n\n[Guide](guide.md)\n")
	writeTestFile(t, root, "content/guide.md", "# Guide\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "content baseline")

	runGit(t, root, "mv", "content/guide.md", "content/moved.md")
	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "content", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "content")
	found := false
	for _, violation := range step.Violations {
		if violation.Rule == "content-link" && violation.File == "content/index.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("content violations = %#v, want the broken guide link", step.Violations)
	}
	paths := strings.Join(report.Scope.ChangedFiles, ",")
	if !strings.Contains(paths, "content/guide.md") || !strings.Contains(paths, "content/moved.md") {
		t.Fatalf("changed files = %#v, want both rename paths", report.Scope.ChangedFiles)
	}

	runGit(t, root, "rm", "-q", "-f", "content/moved.md")
	deleted := runVerifyInProject(t, root, "verify", "--changed", "--only", "content", "--json")
	assertVerifyExitCode(t, deleted, output.ExitVerification)
}

func TestVerifyChangedRunsContentForDeclaredCatalogFileChange(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, contentManifestName, `schemaVersion = 1
include = ["content/**/*.md"]
catalogFile = "content/catalog.toml"
`)
	writeTestFile(t, root, "content/catalog.toml", `schemaVersion = 1

[[entry]]
slug = "guide"
file = "content/guide.md"
`)
	writeTestFile(t, root, "content/index.md", "# Home\n\n[Guide](guide.md)\n")
	writeTestFile(t, root, "content/guide.md", "# Guide\n")
	writeTestFile(t, root, "content/other.md", "# Other\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "content baseline")

	// Only the declared catalog file changes: no Markdown, manifest, deletion,
	// or rename signals that content invariants could be affected.
	writeTestFile(t, root, "content/catalog.toml", `schemaVersion = 1

[[entry]]
slug = "guide"
file = "content/guide.md"

[[entry]]
slug = "guide"
file = "content/other.md"
`)
	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "content", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "content")
	found := false
	for _, violation := range step.Violations {
		if violation.Rule == "content-slug" && violation.File == contentManifestName {
			found = true
		}
	}
	if !found {
		t.Fatalf("content violations = %#v, want the duplicate catalog slug", step.Violations)
	}
	widened := false
	for _, widening := range report.Scope.Widened {
		if widening.Step == "content" {
			widened = true
		}
	}
	if !widened {
		t.Fatalf("scope.widened = %#v, want the content stage", report.Scope.Widened)
	}
}

func TestVerifyChangedRunsContentForRequiredInputChange(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, contentManifestName, `schemaVersion = 1
include = ["content/**/*.md"]
required = ["content/required.txt"]
`)
	writeTestFile(t, root, "content/index.md", "# Home\n")
	writeTestFile(t, root, "content/required.txt", "first\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "content baseline")

	// required declares a non-Markdown input: changing it must not leave the
	// content stage skipped just because no Markdown or manifest changed.
	writeTestFile(t, root, "content/required.txt", "second\n")
	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "content", "--json")
	if result.err != nil {
		t.Fatalf("verify --changed --only content: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "content")
	if step.Status != verifyStatusOK || !step.OK {
		t.Fatalf("content step = %#v, want the required-input change checked", step)
	}
	widened := false
	for _, widening := range report.Scope.Widened {
		if widening.Step == "content" {
			widened = true
		}
	}
	if !widened {
		t.Fatalf("scope.widened = %#v, want the content stage", report.Scope.Widened)
	}
}

func TestVerifyChangedRunsContentWhenOpaqueCheckerInputsChange(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, contentManifestName, `schemaVersion = 1
include = ["content/**/*.md"]

[[checkers]]
name = "docs-check"
run = ["sh", "-c", "test ! -f broken.txt"]
`)
	writeTestFile(t, root, "content/index.md", "# Home\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "content baseline")

	// A checker's real inputs are opaque to the verifier: an unrelated file can
	// change what it reads or how it behaves, so content invariants must run.
	writeTestFile(t, root, "broken.txt", "checker input changed\n")
	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "content", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "content")
	found := false
	for _, violation := range step.Violations {
		if violation.Rule == "content-checker" {
			found = true
		}
	}
	if !found {
		t.Fatalf("content violations = %#v, want the checker failure", step.Violations)
	}
}

func TestVerifyChangedIncludesExternalTestImportDependents(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, ".gitignore", "bin/\n")
	writeTestFile(t, root, "a/a.go", "package a\n\nfunc Value() int { return 1 }\n")
	writeTestFile(t, root, "b/b.go", "package b\n\nfunc B() {}\n")
	writeTestFile(t, root, "b/b_test.go", `package b_test

import (
	"testing"

	"example.com/app/a"
)

func TestValue(t *testing.T) {
	if a.Value() != 1 {
		t.Fatalf("value = %d", a.Value())
	}
}
`)
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "baseline")

	// The external test package b_test is the only importer of a, so b is a
	// reverse dependency through XTestImports rather than Imports/TestImports.
	writeTestFile(t, root, "a/a.go", "package a\n")
	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "go-test", "--with-tests", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T, want verifyReport", output.Fail(result.err).Data)
	}
	packages := strings.Join(report.Scope.GoPackages, ",")
	if !strings.Contains(packages, "example.com/app/a") || !strings.Contains(packages, "example.com/app/b") {
		t.Fatalf("scope.go_packages = %#v, want the changed package and its external test dependent", report.Scope.GoPackages)
	}
	step := findVerifyStep(t, report, "go test")
	if step.Status != verifyStatusFail {
		t.Fatalf("go test step = %#v, want the b_test build failure", step)
	}
}

func TestVerifyChangedSkipsUnaffectedWholeProjectStages(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, "README.md", "# Notes\n")

	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "generated", "--json")
	if result.err != nil {
		t.Fatalf("verify --changed --only generated: %v", result.err)
	}
	report := decodeVerifyReport(t, result)
	generated := findVerifyStep(t, report, "generated")
	if generated.Status != verifyStatusSkipped ||
		!strings.Contains(generated.Reason, "no changed generation inputs") {
		t.Fatalf("generated step = %#v, want skipped for a docs-only change", generated)
	}
	setup := findVerifyStep(t, report, "setup")
	if setup.Status != verifyStatusSkipped {
		t.Fatalf("setup step = %#v, want skipped when generated has no work", setup)
	}
}

func TestVerifyChangedGeneratedInputRunsWholeProjectStep(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, "handlers/pages.go", "package handlers\n")

	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "generated", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	generated := findVerifyStep(t, report, "generated")
	if generated.Status != verifyStatusBlocked {
		t.Fatalf("generated step = %#v, want blocked by missing tools", generated)
	}
	widened := false
	for _, widening := range report.Scope.Widened {
		if widening.Step == "generated" {
			widened = true
		}
	}
	if !widened {
		t.Fatalf("scope.widened = %#v, want generated widening", report.Scope.Widened)
	}
}

func TestVerifyChangedFrontendScope(t *testing.T) {
	root := newGitVerifyProject(t)
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)
	runGit(t, root, "add", "package.json")
	runGit(t, root, "commit", "-m", "frontend baseline")
	if err := os.MkdirAll(root+"/node_modules", 0o755); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "pnpm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeTestFile(t, root, "resources/js/App.ts", "export const app = true\n")
	result := runVerifyInProject(t, root, "verify", "--changed", "--only", "frontend", "--json")
	if result.err != nil {
		t.Fatalf("verify --changed --only frontend: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "frontend")
	if step.Status != verifyStatusOK {
		t.Fatalf("frontend step = %#v, want ok", step)
	}
	widened := false
	for _, widening := range report.Scope.Widened {
		if widening.Step == "frontend" {
			widened = true
		}
	}
	if !widened {
		t.Fatalf("scope.widened = %#v, want frontend widening", report.Scope.Widened)
	}
}

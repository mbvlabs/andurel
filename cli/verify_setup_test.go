package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func TestVerifySetupBlocksFrontendUntilNodeModulesInstalled(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	setup := findVerifyStep(t, report, "setup")
	if setup.Status != verifyStatusFail || len(setup.Violations) != 1 {
		t.Fatalf("setup step = %#v, want one setup failure", setup)
	}
	if setup.Violations[0].Rule != "setup" || setup.Violations[0].Fix != "pnpm install" {
		t.Fatalf("setup violation = %#v, want setup rule and pnpm install", setup.Violations[0])
	}
	frontend := findVerifyStep(t, report, "frontend")
	if frontend.Status != verifyStatusBlocked || frontend.OK {
		t.Fatalf("frontend step = %#v, want blocked", frontend)
	}
	if len(frontend.Violations) != 0 {
		t.Fatalf("blocked frontend must not report typecheck drift: %#v", frontend.Violations)
	}
	if !strings.Contains(frontend.Detail, "pnpm install") {
		t.Fatalf("blocked frontend detail = %q, want the install command", frontend.Detail)
	}

	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "pnpm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	green := runVerifyInProject(t, root, "verify", "--skip-generated", "--json")
	if green.err != nil {
		t.Fatalf("verify after setup: %v\n%s", green.err, green.stderr)
	}
	report = decodeVerifyReport(t, green)
	setup = findVerifyStep(t, report, "setup")
	if setup.Status != verifyStatusOK {
		t.Fatalf("setup after install = %#v, want ok", setup)
	}
	frontend = findVerifyStep(t, report, "frontend")
	if frontend.Status != verifyStatusOK {
		t.Fatalf("frontend after install = %#v, want ok", frontend)
	}
}

func TestVerifySetupHumanShowsBlockedStages(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify")
	assertVerifyExitCode(t, result, output.ExitVerification)
	for _, want := range []string{
		"setup      FAIL",
		"generated  BLOCK",
		"andurel tool sync",
	} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("human output is missing %q:\n%s", want, result.stdout)
		}
	}
	if strings.Contains(result.stdout, "[generated]") {
		t.Fatalf("missing tools must not be reported as generated drift:\n%s", result.stdout)
	}
}

func TestVerifySetupSkippedForContentOnlyRuns(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
	if result.err != nil {
		t.Fatalf("verify --only content: %v", result.err)
	}
	report := decodeVerifyReport(t, result)
	setup := findVerifyStep(t, report, "setup")
	if setup.Status != verifyStatusSkipped {
		t.Fatalf("setup step = %#v, want skipped", setup)
	}
	if !strings.Contains(setup.Reason, "no setup requirements") {
		t.Fatalf("setup reason = %q", setup.Reason)
	}
}

func TestVerifyExplicitSetupChecksFullProject(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--only", "setup", "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	setup := findVerifyStep(t, report, "setup")
	if setup.Status != verifyStatusFail || len(setup.Violations) < 2 {
		t.Fatalf("setup step = %#v, want the full project preflight", setup)
	}
}

func TestVerifyFrontendStageReportsRuleSelection(t *testing.T) {
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "package.json", `{"scripts":{"typecheck":"tsc --noEmit"}}`)
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	fakeBin := t.TempDir()
	writeExecutable(t, fakeBin, "pnpm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	result := runVerifyInProject(t, root, "verify", "--only", "frontend", "--rules", "frontend", "--json")
	if result.err != nil {
		t.Fatalf("verify --rules frontend: %v", result.err)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "frontend")
	if step.Rules == nil {
		t.Fatalf("frontend step has no rule report: %#v", step)
	}
	if len(step.Rules.Selected) != 1 || step.Rules.Selected[0] != "frontend" {
		t.Fatalf("frontend rules = %#v, want frontend selected", step.Rules)
	}
	if len(step.Rules.Executed) != 1 || step.Rules.Executed[0] != "frontend" {
		t.Fatalf("frontend rules = %#v, want frontend executed", step.Rules)
	}
	if len(step.Rules.Skipped) != 0 {
		t.Fatalf("frontend rules = %#v, want no selected-but-skipped rules", step.Rules)
	}
}

package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func stubFrontendSetupNode(t *testing.T, report string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell executable")
	}
	bin := t.TempDir()
	writeTestFile(t, bin, "node", "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '"+report+"'\n")
	if err := os.Chmod(filepath.Join(bin, "node"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestFrontendSetupConfigReportIgnoresPluginLogs(t *testing.T) {
	stubFrontendSetupNode(t, "untrusted plugin log\nANDUREL_FRONTEND_SETUP={\"violations\":[]}")
	result := runFrontendSetupConfig(t.TempDir())
	if result.Blocked != "" || len(result.Violations) != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestFrontendSetupConfigMissingNodeIsBlocked(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	result := runFrontendSetupConfig(t.TempDir())
	if !strings.Contains(result.Blocked, "Node.js is missing") {
		t.Fatalf("result = %#v", result)
	}
}

func TestFrontendSetupConfigInvalidReportIsBlockedWithoutLogs(t *testing.T) {
	stubFrontendSetupNode(t, "untrusted plugin log")
	result := runFrontendSetupConfig(t.TempDir())
	if result.Blocked == "" || strings.Contains(result.Blocked, "untrusted") {
		t.Fatalf("result = %#v", result)
	}
}

func TestFrontendSetupConfigMissingDependenciesNamesInstallCommand(t *testing.T) {
	stubFrontendSetupNode(t, `ANDUREL_FRONTEND_SETUP={"violations":[],"blocked":"Vite is not installed","missingDependencies":true}`)
	result := runFrontendSetupConfig(t.TempDir())
	if !strings.Contains(result.Blocked, "pnpm install") {
		t.Fatalf("result = %#v", result)
	}
}

func TestFrontendSetupEnvironmentMatchesRunnerWithoutGlobalMutation(t *testing.T) {
	root := t.TempDir()
	const key = "ANDUREL_FRONTEND_SETUP_TEST_VALUE"
	t.Setenv(key, "external")
	writeTestFile(t, root, ".env", key+"=file\nANDUREL_FRONTEND_SETUP_TEST_FILE_ONLY=file-only\n")
	env, err := frontendSetupEnvironment(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, key+"=external") || !slices.Contains(env, "ANDUREL_FRONTEND_SETUP_TEST_FILE_ONLY=file-only") ||
		slices.Contains(env, key+"=file") {
		t.Fatalf("environment does not match runner precedence")
	}
	if _, exists := os.LookupEnv("ANDUREL_FRONTEND_SETUP_TEST_FILE_ONLY"); exists {
		t.Fatal("frontend setup mutated the process environment")
	}
}

func TestVerifyFrontendSetupRuleNotApplicable(t *testing.T) {
	root := newVerifyTestProject(t)
	result := runVerifyInProject(t, root, "verify", "--rules", frontendSetupRule, "--json")
	if result.err != nil {
		t.Fatalf("verify: %v\n%s", result.err, result.stderr)
	}
	step := findVerifyStep(t, decodeVerifyReport(t, result), "frontend")
	if step.Status != verifyStatusSkipped || step.Rules == nil ||
		!slices.Contains(step.Rules.Selected, frontendSetupRule) ||
		slices.Contains(step.Rules.Executed, frontendSetupRule) {
		t.Fatalf("step = %#v", step)
	}
}

func TestVerifyFrontendSetupRuleReportsActionableViolation(t *testing.T) {
	stubFrontendSetupNode(t, `ANDUREL_FRONTEND_SETUP={"violations":[{"rule":"frontend-setup","file":"vite.config.ts","message":"development origin does not match Vite","fix":"derive server.origin from INERTIA_VITE_DEV_URL"}]}`)
	root := newVerifyTestProject(t)
	writeTestFile(t, root, "vite.config.ts", "export default {}\n")
	writeTestFile(t, root, "package.json", `{"dependencies":{"@inertiajs/react":"3.6.1"}}`)
	result := runVerifyInProject(t, root, "verify", "--rules", frontendSetupRule, "--json")
	assertVerifyExitCode(t, result, output.ExitVerification)
	report, ok := output.Fail(result.err).Data.(verifyReport)
	if !ok {
		t.Fatalf("failure data = %T", output.Fail(result.err).Data)
	}
	step := findVerifyStep(t, report, "frontend")
	if step.Status != verifyStatusFail || step.Rules == nil ||
		!slices.Contains(step.Rules.Executed, frontendSetupRule) {
		t.Fatalf("step = %#v", step)
	}
	found := false
	for _, violation := range step.Violations {
		if violation.File == "vite.config.ts" && strings.Contains(violation.Message, "development origin") {
			found = violation.Rule == frontendSetupRule && violation.Fix != ""
		}
	}
	if !found {
		t.Fatalf("missing actionable config violation: %#v", step)
	}
}

func TestFrontendSetupChangedGoWiringTriggersFrontend(t *testing.T) {
	for _, path := range []string{
		"routes/assets.go", "handlers/assets.go", "app/runtime.go", "assets/assets.go",
		"css/base.css", "vite.config.ts", "andurel.lock",
		"vite.config.mts", "vite.config.cts", "config/hosts.go",
		"pnpm-lock.yaml", "package-lock.json",
	} {
		if !changedFilesAffectFrontend([]verifyChangedEntry{{Path: path}}) {
			t.Errorf("%s does not trigger frontend verification", path)
		}
	}
}

func TestFrontendSetupUnrelatedViteIsNotApplicable(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "vite.config.ts", "export default {}\n")
	writeTestFile(t, root, "package.json", `{"devDependencies":{"vite":"7.3.8"}}`)
	if frontendSetupApplicable(root) {
		t.Fatal("unrelated Vite project opted into the Inertia setup contract")
	}
}

func TestVerifyFrontendSetupGeneratedScaffolds(t *testing.T) {
	for _, adapter := range []string{"react", "vue", "svelte"} {
		t.Run(adapter, func(t *testing.T) {
			root := newVerifyTestProject(t)
			golden := filepath.Join("..", "testdata", "golden", "new", "postgresql-inertia-"+adapter)
			err := filepath.WalkDir(golden, func(file string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() || !strings.HasSuffix(file, ".golden") {
					return nil
				}
				relative, err := filepath.Rel(golden, file)
				if err != nil {
					return err
				}
				relative = strings.TrimSuffix(relative, ".golden")
				if !strings.HasSuffix(relative, ".go") && relative != "package.json" &&
					relative != filepath.Join("css", "base.css") &&
					relative != filepath.Join("resources", "js", "app.ts") &&
					relative != filepath.Join("resources", "js", "app.tsx") {
					return nil
				}
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				writeTestFile(t, root, relative, string(data))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
				t.Fatal(err)
			}
			entry := "resources/js/app.ts"
			if adapter == "react" {
				entry += "x"
			}
			t.Setenv("INERTIA_ENTRY_POINT", entry)
			report, err := json.Marshal(frontendSetupResult{EntryPoints: []string{filepath.Join(root, filepath.FromSlash(entry))}})
			if err != nil {
				t.Fatal(err)
			}
			// The actual Vite resolver has a separate real-Vite matrix. Here
			// exercise complete CLI aggregation against each generated Go/CSS
			// scaffold without requiring npm installation in Go unit tests.
			stubFrontendSetupNode(t, "ANDUREL_FRONTEND_SETUP="+string(report))
			result := runVerifyInProject(t, root, "verify", "--rules", frontendSetupRule, "--json")
			if result.err != nil {
				t.Fatalf("canonical %s scaffold rejected: %v\n%s", adapter, result.err, result.stderr)
			}
			step := findVerifyStep(t, decodeVerifyReport(t, result), "frontend")
			if step.Status != verifyStatusOK {
				t.Fatalf("step = %#v", step)
			}
			t.Setenv("INERTIA_ENTRY_POINT", "resources/js/not-a-build-input.ts")
			result = runVerifyInProject(t, root, "verify", "--rules", frontendSetupRule, "--json")
			assertVerifyExitCode(t, result, output.ExitVerification)
			failure, ok := output.Fail(result.err).Data.(verifyReport)
			if !ok {
				t.Fatalf("failure data = %T", output.Fail(result.err).Data)
			}
			step = findVerifyStep(t, failure, "frontend")
			found := false
			for _, violation := range step.Violations {
				if violation.Message == "renderer EntryPoint is not a Vite production input" {
					found = true
				}
			}
			if !found {
				t.Fatalf("entry override mismatch was not diagnosed: %#v", step)
			}
		})
	}
}

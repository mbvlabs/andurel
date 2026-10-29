package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

func stubProjectConfigCheck(
	t *testing.T,
	report configCheckReport,
	err error,
) *[]string {
	t.Helper()

	var roots []string
	original := runProjectConfigCheckFunc
	runProjectConfigCheckFunc = func(rootDir string) (configCheckReport, error) {
		roots = append(roots, rootDir)
		return report, err
	}
	t.Cleanup(func() {
		runProjectConfigCheckFunc = original
	})

	return &roots
}

func writeRuntimeConfigProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, configCheckPath, "package main\n\nfunc main() {}\n")

	return root
}

func TestCheckRuntimeConfigurationSkipsWithoutHelper(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)

	got := checkRuntimeConfiguration(root)
	if got.status != statusSkip {
		t.Fatalf("missing helper check = %#v, want skip", got)
	}
	if !strings.Contains(got.hint, configCheckPath) {
		t.Fatalf("skip hint does not name the helper: %q", got.hint)
	}
}

func TestCheckRuntimeConfigurationReportsEachOutcome(t *testing.T) {
	t.Run("valid configuration and reachable database", func(t *testing.T) {
		root := writeRuntimeConfigProject(t)
		roots := stubProjectConfigCheck(t, configCheckReport{
			OK:     true,
			Config: configCheckSection{Status: "valid"},
			Database: &configCheckSection{
				Status: "reachable",
			},
		}, nil)

		got := checkRuntimeConfiguration(root)
		if got.status != statusPass {
			t.Fatalf("reachable check = %#v, want pass", got)
		}
		if len(*roots) != 1 || (*roots)[0] != root {
			t.Fatalf("config check ran against %v, want %q", *roots, root)
		}
	})

	t.Run("missing or invalid configuration blocks", func(t *testing.T) {
		root := writeRuntimeConfigProject(t)
		stubProjectConfigCheck(t, configCheckReport{
			Config: configCheckSection{
				Status: "invalid",
				Errors: []string{"config: app: HOST_PRIMARY is required"},
			},
		}, nil)

		got := checkRuntimeConfiguration(root)
		if got.status != statusFail {
			t.Fatalf("invalid config check = %#v, want fail", got)
		}
		if len(got.details) != 1 ||
			!strings.Contains(got.details[0], "HOST_PRIMARY is required") {
			t.Fatalf("invalid config details = %v", got.details)
		}
		if !strings.Contains(got.hint, ".env.example") {
			t.Fatalf("invalid config hint = %q", got.hint)
		}
	})

	t.Run("unreachable database blocks", func(t *testing.T) {
		root := writeRuntimeConfigProject(t)
		stubProjectConfigCheck(t, configCheckReport{
			Config: configCheckSection{Status: "valid"},
			Database: &configCheckSection{
				Status: "unreachable",
				Reason: "storage: ping database: connection refused",
			},
		}, nil)

		got := checkRuntimeConfiguration(root)
		if got.status != statusFail || got.message != "database is unreachable" {
			t.Fatalf("unreachable database check = %#v, want fail", got)
		}
		if len(got.details) != 1 || !strings.Contains(got.details[0], "connection refused") {
			t.Fatalf("unreachable database details = %v", got.details)
		}
	})

	t.Run("database probe skipped warns instead of passing", func(t *testing.T) {
		root := writeRuntimeConfigProject(t)
		stubProjectConfigCheck(t, configCheckReport{
			Config: configCheckSection{Status: "valid"},
			Database: &configCheckSection{
				Status: "skipped",
				Reason: "no database is configured",
			},
		}, nil)

		got := checkRuntimeConfiguration(root)
		if got.status != statusWarn {
			t.Fatalf("skipped database check = %#v, want warn", got)
		}
	})

	t.Run("check that cannot run is skipped with the build error", func(t *testing.T) {
		root := writeRuntimeConfigProject(t)
		stubProjectConfigCheck(
			t,
			configCheckReport{},
			errors.New("exit status 1: internal/configcheck/main.go:4:2: no required module"),
		)

		got := checkRuntimeConfiguration(root)
		if got.status != statusSkip {
			t.Fatalf("build failure check = %#v, want skip", got)
		}
		if len(got.details) != 1 || !strings.Contains(got.details[0], "no required module") {
			t.Fatalf("build failure details = %v", got.details)
		}
	})
}

func TestRunProjectConfigCheckBuildFailureKeepsStderr(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, configCheckPath, "package main\n\nfunc main() { this is not Go }\n")

	_, err := runProjectConfigCheck(root)
	if err == nil {
		t.Fatal("expected a build failure for an unparsable helper")
	}
	if !strings.Contains(err.Error(), "configcheck") && !strings.Contains(err.Error(), "syntax") {
		t.Fatalf("build failure error = %v", err)
	}
}

func TestCheckProjectSetupReportsMissingToolsAndFrontendDependencies(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	lock := layout.NewAndurelLock("v1.2.3")
	lock.Tools["templ"] = validTestTool("templ", "v0.3.1")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:              "app",
		Inertia:                  "react",
		JavaScriptPackageManager: "pnpm",
	}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	fresh := checkProjectSetup(root)
	if fresh.status != statusWarn || fresh.message != "setup is incomplete" {
		t.Fatalf("fresh setup check = %#v, want warn", fresh)
	}
	if !strings.Contains(fresh.hint, "andurel tool sync && pnpm install") {
		t.Fatalf("fresh setup hint = %q, want the exact commands", fresh.hint)
	}
	if len(fresh.details) != 2 {
		t.Fatalf("fresh setup details = %v, want missing tools and node_modules", fresh.details)
	}

	writeExecutable(t, root, "bin/templ", "#!/bin/sh\nexit 0\n")
	writeTestFile(t, root, "node_modules/.keep", "")

	complete := checkProjectSetup(root)
	if complete.status != statusPass {
		t.Fatalf("complete setup check = %#v, want pass", complete)
	}
}

func TestDetectSetupStateUsesRecordedPackageManager(t *testing.T) {
	tests := []struct {
		manager  string
		install  string
		lockfile string
	}{
		{manager: "pnpm", install: "pnpm install", lockfile: "pnpm-lock.yaml"},
		{manager: "npm", install: "npm install", lockfile: "package-lock.json"},
		{manager: "bun", install: "bun install", lockfile: "bun.lock"},
	}

	for _, test := range tests {
		t.Run(test.manager, func(t *testing.T) {
			root := t.TempDir()
			writeGoModule(t, root)
			lock := layout.NewAndurelLock("v1.2.3")
			lock.ScaffoldConfig = &layout.ScaffoldConfig{
				ProjectName:              "app",
				Inertia:                  "react",
				JavaScriptPackageManager: test.manager,
			}
			if err := lock.WriteLockFile(root); err != nil {
				t.Fatalf("write lock: %v", err)
			}

			state, err := detectSetupState(root)
			if err != nil {
				t.Fatalf("detect setup state: %v", err)
			}
			if state.InstallCommand() != test.install {
				t.Fatalf("install command = %q, want %q", state.InstallCommand(), test.install)
			}

			data := &layout.TemplateData{
				ProjectName:              "app",
				JavaScriptPackageManager: test.manager,
			}
			if got := data.JavaScriptLockfileName(); got != test.lockfile {
				t.Fatalf("lockfile = %q, want %q", got, test.lockfile)
			}
			if got := data.JavaScriptFrozenInstallCommand(); got == "" {
				t.Fatalf("frozen install command is empty for %s", test.manager)
			}
		})
	}
}

func TestDetectSetupStateFailsOnInvalidLock(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	if err := os.WriteFile(filepath.Join(root, layout.ProjectTomlName), []byte("schemaVersion = \""), 0o644); err != nil {
		t.Fatalf("write invalid lock: %v", err)
	}

	if _, err := detectSetupState(root); err == nil {
		t.Fatal("expected an error for an invalid andurel.lock")
	}
	got := checkProjectSetup(root)
	if got.status != statusSkip {
		t.Fatalf("invalid lock setup check = %#v, want skip", got)
	}
}

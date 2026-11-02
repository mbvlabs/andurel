package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
	"github.com/spf13/cobra"
)

type upgradeJSONEnvelope struct {
	OK          bool                `json:"ok"`
	Summary     string              `json:"summary"`
	Breadcrumbs []output.Breadcrumb `json:"breadcrumbs"`
	Data        struct {
		Artifacts mutationReport `json:"artifacts"`
	} `json:"data"`
}

type upgradeJSONFake struct {
	report *upgrade.UpgradeReport
	err    error
	opts   upgrade.UpgradeOptions
	write  func()
}

func (f *upgradeJSONFake) Execute() (*upgrade.UpgradeReport, error) {
	if f.write != nil && !f.opts.DryRun {
		f.write()
	}
	if f.report != nil {
		return f.report, f.err
	}
	return &upgrade.UpgradeReport{}, f.err
}

func prepareUpgradeJSONTest(t *testing.T, root string) {
	t.Helper()
	resetCLITestSeams(t)
	stubLatestAndurelVersion(t, "v2.0.0", nil)
	findGoModRoot = func() (string, error) { return root, nil }
}

func newUpgradeJSONCommand(t *testing.T, out *bytes.Buffer, dryRun, repair bool) *cobra.Command {
	t.Helper()

	cmd := newStructuredTestCommand(out)
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")
	cmd.Flags().Bool("repair", false, "")
	for _, flag := range []struct {
		name  string
		value bool
	}{
		{name: "dry-run", value: dryRun},
		{name: "repair", value: repair},
	} {
		if err := cmd.Flags().Set(flag.name, boolString(flag.value)); err != nil {
			t.Fatalf("set %s: %v", flag.name, err)
		}
	}
	return cmd
}

func newUpgradeHumanCommand(out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{Use: "upgrade"}
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")
	cmd.Flags().Bool("repair", false, "")
	cmd.SetOut(out)
	return cmd
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func runUpgradeJSON(t *testing.T, cmd *cobra.Command, out *bytes.Buffer) upgradeJSONEnvelope {
	t.Helper()

	if err := runUpgrade(cmd, "v2.0.0"); err != nil {
		t.Fatalf("runUpgrade: %v\n%s", err, out.String())
	}
	var envelope upgradeJSONEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode upgrade envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK {
		t.Fatalf("upgrade envelope is not ok:\n%s", out.String())
	}
	return envelope
}

func upgradeBreadcrumbCommands(breadcrumbs []output.Breadcrumb) []string {
	commands := make([]string, 0, len(breadcrumbs))
	for _, breadcrumb := range breadcrumbs {
		commands = append(commands, breadcrumb.Command)
	}
	return commands
}

func upgradeWarningsContain(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}

func TestStructuredUpgradeReportsOnlyExecutedCommands(t *testing.T) {
	for _, tc := range []struct {
		name          string
		report        *upgrade.UpgradeReport
		writeGoMod    bool
		wantTidy      bool
		wantToolSync  bool
		wantGoModFile bool
	}{
		{
			name: "module and tool changes",
			report: &upgrade.UpgradeReport{
				Success:       true,
				ReplacedFiles: []string{"go.mod"},
				ToolsUpdated:  1,
			},
			writeGoMod:    true,
			wantTidy:      true,
			wantToolSync:  true,
			wantGoModFile: true,
		},
		{
			name: "module changes only",
			report: &upgrade.UpgradeReport{
				Success:       true,
				ReplacedFiles: []string{"go.mod"},
			},
			writeGoMod:    true,
			wantTidy:      true,
			wantGoModFile: true,
		},
		{
			name: "tool changes only",
			report: &upgrade.UpgradeReport{
				Success:      true,
				ToolsUpdated: 1,
			},
			wantToolSync: true,
		},
		{
			name: "no changes",
			report: &upgrade.UpgradeReport{
				Success: true,
			},
		},
		{
			name: "already current",
			report: &upgrade.UpgradeReport{
				Success:        true,
				AlreadyCurrent: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeGoModule(t, root)
			writeTestFile(t, root, "andurel.toml", "schemaVersion = 1\nversion = \"v1.0.0\"\n")
			prepareUpgradeJSONTest(t, root)

			fake := &upgradeJSONFake{report: tc.report}
			if tc.writeGoMod {
				fake.write = func() {
					writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n\nrequire example.com/new v1.0.0\n")
				}
			}
			newUpgraderFunc = func(_ string, opts upgrade.UpgradeOptions) (cliUpgrader, error) {
				fake.opts = opts
				return fake, nil
			}
			syncCalls := 0
			syncSingleToolFunc = func(string, string, *layout.Tool, string, string) error {
				syncCalls++
				return nil
			}

			var out bytes.Buffer
			cmd := newUpgradeJSONCommand(t, &out, false, false)
			envelope := runUpgradeJSON(t, cmd, &out)
			artifacts := envelope.Data.Artifacts

			if fake.opts.DryRun {
				t.Fatal("real upgrade was initialized as a dry run")
			}
			if len(artifacts.CommandsRun) != 0 {
				t.Fatalf("commands_run = %#v, want no executed commands", artifacts.CommandsRun)
			}
			if !strings.Contains(out.String(), `"commands_run": []`) {
				t.Fatalf("commands_run is not an empty array:\n%s", out.String())
			}
			if syncCalls != 0 {
				t.Fatalf("structured upgrade ran %d tool syncs; it must not execute follow-up work", syncCalls)
			}
			if got := upgradeWarningsContain(artifacts.Warnings, "go mod tidy"); got != tc.wantTidy {
				t.Fatalf("go mod tidy warning = %v, want %v; warnings=%#v", got, tc.wantTidy, artifacts.Warnings)
			}
			if got := upgradeWarningsContain(artifacts.Warnings, "andurel tool sync"); got != tc.wantToolSync {
				t.Fatalf("tool sync warning = %v, want %v; warnings=%#v", got, tc.wantToolSync, artifacts.Warnings)
			}
			breadcrumbs := upgradeBreadcrumbCommands(envelope.Breadcrumbs)
			if got := slices.Contains(breadcrumbs, "go mod tidy"); got != tc.wantTidy {
				t.Fatalf("go mod tidy breadcrumb = %v, want %v; breadcrumbs=%#v", got, tc.wantTidy, breadcrumbs)
			}
			if got := slices.Contains(breadcrumbs, "andurel tool sync"); got != tc.wantToolSync {
				t.Fatalf("tool sync breadcrumb = %v, want %v; breadcrumbs=%#v", got, tc.wantToolSync, breadcrumbs)
			}
			if got := slices.Contains(artifacts.FilesUpdated, "go.mod"); got != tc.wantGoModFile {
				t.Fatalf("go.mod updated = %v, want %v; files_updated=%#v", got, tc.wantGoModFile, artifacts.FilesUpdated)
			}
		})
	}
}

func TestStructuredUpgradeReportsDomainCommandProvenance(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "andurel.toml", "schemaVersion = 1\nversion = \"v1.0.0\"\n")
	prepareUpgradeJSONTest(t, root)

	executed := []string{
		"git status --porcelain",
		"git status --porcelain",
	}
	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return &upgradeJSONFake{report: &upgrade.UpgradeReport{
			Success:     true,
			CommandsRun: executed,
		}}, nil
	}

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, false, false)
	envelope := runUpgradeJSON(t, cmd, &out)
	if got := envelope.Data.Artifacts.CommandsRun; !slices.Equal(got, executed) {
		t.Fatalf("commands_run = %#v, want the domain's executed commands %#v", got, executed)
	}
	if strings.Contains(out.String(), `"commands_run": []`) {
		t.Fatalf("actual domain commands were dropped:\n%s", out.String())
	}
}

func TestStructuredUpgradeDryRunProposesFollowUpsWithoutExecution(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "andurel.toml", "schemaVersion = 1\nversion = \"v1.0.0\"\n")
	prepareUpgradeJSONTest(t, root)

	fake := &upgradeJSONFake{report: &upgrade.UpgradeReport{
		Success:       true,
		ReplacedFiles: []string{"go.mod"},
		ToolsUpdated:  1,
	}}
	newUpgraderFunc = func(_ string, opts upgrade.UpgradeOptions) (cliUpgrader, error) {
		fake.opts = opts
		return fake, nil
	}
	syncCalls := 0
	syncSingleToolFunc = func(string, string, *layout.Tool, string, string) error {
		syncCalls++
		return nil
	}

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, true, false)
	envelope := runUpgradeJSON(t, cmd, &out)
	artifacts := envelope.Data.Artifacts

	if !fake.opts.DryRun {
		t.Fatal("dry-run flag was not passed to the upgrader")
	}
	if syncCalls != 0 {
		t.Fatalf("dry run ran %d tool syncs", syncCalls)
	}
	if len(artifacts.CommandsRun) != 0 {
		t.Fatalf("dry-run commands_run = %#v, want no executed commands", artifacts.CommandsRun)
	}
	for _, want := range []string{
		"dry run",
		"go mod tidy",
		"was not run",
		"andurel tool sync",
	} {
		if !upgradeWarningsContain(artifacts.Warnings, want) {
			t.Fatalf("dry-run warnings missing %q: %#v", want, artifacts.Warnings)
		}
	}
	for _, want := range []string{"go mod tidy", "andurel tool sync"} {
		if !slices.Contains(upgradeBreadcrumbCommands(envelope.Breadcrumbs), want) {
			t.Fatalf("dry-run breadcrumbs missing %q: %#v", want, envelope.Breadcrumbs)
		}
	}
}

func TestStructuredUpgradeRepairAndDriftWarningsKeepCommandsEmpty(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "andurel.toml", "schemaVersion = 1\nversion = \"v1.0.0\"\n")
	prepareUpgradeJSONTest(t, root)

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return &upgradeJSONFake{report: &upgrade.UpgradeReport{
			Success:       true,
			RepairApplied: true,
			FilesReplaced: 1,
		}}, nil
	}

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, false, true)
	envelope := runUpgradeJSON(t, cmd, &out)
	artifacts := envelope.Data.Artifacts
	if len(artifacts.CommandsRun) != 0 {
		t.Fatalf("repair commands_run = %#v, want no executed commands", artifacts.CommandsRun)
	}
	if upgradeWarningsContain(artifacts.Warnings, "go mod tidy") ||
		upgradeWarningsContain(artifacts.Warnings, "andurel tool sync") {
		t.Fatalf("repair without module or tool changes reported stale follow-ups: %#v", artifacts.Warnings)
	}
	if upgradeWarningsContain(artifacts.Warnings, "andurel upgrade --repair") {
		t.Fatalf("applied repair still reported pending drift: %#v", artifacts.Warnings)
	}

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return &upgradeJSONFake{report: &upgrade.UpgradeReport{
			Success:         true,
			RepairAvailable: true,
			ReplacedFiles:   []string{"handlers/pages.go"},
		}}, nil
	}
	out.Reset()
	cmd = newUpgradeJSONCommand(t, &out, false, false)
	envelope = runUpgradeJSON(t, cmd, &out)
	artifacts = envelope.Data.Artifacts
	if len(artifacts.CommandsRun) != 0 {
		t.Fatalf("drift report commands_run = %#v, want no executed commands", artifacts.CommandsRun)
	}
	if !upgradeWarningsContain(artifacts.Warnings, "andurel upgrade --repair") {
		t.Fatalf("drift warning missing repair command: %#v", artifacts.Warnings)
	}
}

func TestStructuredUpgradePendingRepairUsesProposalWording(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "andurel.toml", "schemaVersion = 1\nversion = \"v1.0.0\"\n")
	prepareUpgradeJSONTest(t, root)

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return &upgradeJSONFake{report: &upgrade.UpgradeReport{
			Success:         true,
			RepairAvailable: true,
			ReplacedFiles:   []string{"go.mod", "handlers/pages.go"},
			ToolsUpdated:    1,
		}}, nil
	}

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, false, false)
	envelope := runUpgradeJSON(t, cmd, &out)
	artifacts := envelope.Data.Artifacts
	if len(artifacts.Warnings) == 0 ||
		!strings.Contains(artifacts.Warnings[0], "andurel upgrade --repair") {
		t.Fatalf("first warning should advise repair first: %#v", artifacts.Warnings)
	}
	if upgradeWarningsContain(artifacts.Warnings, "go.mod pins changed") {
		t.Fatalf("pending repair reported an unapplied go.mod as changed: %#v", artifacts.Warnings)
	}
	for _, want := range []string{"proposed", "not applied", "go mod tidy", "andurel tool sync"} {
		if !upgradeWarningsContain(artifacts.Warnings, want) {
			t.Fatalf("pending repair warnings missing %q: %#v", want, artifacts.Warnings)
		}
	}
	breadcrumbs := upgradeBreadcrumbCommands(envelope.Breadcrumbs)
	if len(breadcrumbs) == 0 || breadcrumbs[0] != "andurel upgrade --repair" {
		t.Fatalf("repair should be the first breadcrumb: %#v", breadcrumbs)
	}
	for _, want := range []string{"go mod tidy", "andurel tool sync"} {
		if !slices.Contains(breadcrumbs, want) {
			t.Fatalf("pending repair breadcrumbs missing %q: %#v", want, breadcrumbs)
		}
	}
}

func TestStructuredUpgradeFailureDoesNotEmitExecutedCommands(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	prepareUpgradeJSONTest(t, root)

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return &upgradeJSONFake{
			report: &upgrade.UpgradeReport{ReplacedFiles: []string{"go.mod"}, Success: true},
			err:    errors.New("apply failed"),
		}, nil
	}

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, false, false)
	err := runUpgrade(cmd, "v2.0.0")
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("upgrade error = %v, want apply failure", err)
	}
	if out.Len() != 0 {
		t.Fatalf("failed structured upgrade emitted a success envelope:\n%s", out.String())
	}
}

func TestHumanUpgradeSyncsToolsOnlyWhenToolVersionsChange(t *testing.T) {
	for _, tc := range []struct {
		name          string
		toolsUpdated  int
		wantSync      bool
		wantTidyStep  bool
		replacedFiles []string
	}{
		{
			name:          "tool changes",
			toolsUpdated:  1,
			wantSync:      true,
			wantTidyStep:  true,
			replacedFiles: []string{"go.mod"},
		},
		{
			name: "no tool changes",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeGoModule(t, root)
			prepareUpgradeJSONTest(t, root)

			lock := layout.NewAndurelLock("v1.0.0")
			lock.Tools["templ"] = validTestTool("templ", "v0.1.0")
			if err := lock.WriteLockFile(root); err != nil {
				t.Fatalf("write lock: %v", err)
			}

			newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
				return &upgradeJSONFake{report: &upgrade.UpgradeReport{
					Success:       true,
					ReplacedFiles: tc.replacedFiles,
					ToolsUpdated:  tc.toolsUpdated,
				}}, nil
			}
			synced := []string{}
			syncSingleToolFunc = func(_ string, name string, _ *layout.Tool, _, _ string) error {
				synced = append(synced, name)
				return nil
			}

			var out bytes.Buffer
			cmd := newUpgradeHumanCommand(&out)
			if err := runUpgrade(cmd, "v2.0.0"); err != nil {
				t.Fatalf("human runUpgrade: %v", err)
			}
			if got := len(synced) > 0; got != tc.wantSync {
				t.Fatalf("human sync called = %v, want %v; synced=%#v\n%s", got, tc.wantSync, synced, out.String())
			}
			if got := strings.Contains(out.String(), "Syncing tool binaries..."); got != tc.wantSync {
				t.Fatalf("sync progress printed = %v, want %v:\n%s", got, tc.wantSync, out.String())
			}
			if got := strings.Contains(out.String(), "Run 'go mod tidy' to refresh go.sum"); got != tc.wantTidyStep {
				t.Fatalf("tidy step printed = %v, want %v:\n%s", got, tc.wantTidyStep, out.String())
			}
		})
	}
}

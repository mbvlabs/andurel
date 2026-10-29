package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/upgrade"
)

func TestStructuredUpgradeDryRunReportsManifestFiles(t *testing.T) {
	resetCLITestSeams(t)
	stubLatestAndurelVersion(t, "v2.0.0", nil)
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "andurel.toml", "schemaVersion = 1\nversion = \"v1.0.0\"\n")
	writeTestFile(t, root, "internal/old.go", "package internal\n")
	findGoModRoot = func() (string, error) { return root, nil }

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return fakeUpgrader{report: &upgrade.UpgradeReport{
			Success:       true,
			FilesReplaced: 1,
			ReplacedFiles: []string{"go.mod"},
			RemovedFiles:  []string{"internal/old.go"},
			PlannedChanges: []upgrade.PlannedChange{
				{Path: "go.mod", Content: []byte("module example.com/app\n\ngo 1.27.2\n")},
				{Path: "internal/old.go", Remove: true},
				{Path: "andurel.toml", Content: []byte("schemaVersion = 1\nversion = \"v2.0.0\"\n")},
			},
		}}, nil
	}

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")
	for _, flag := range []string{"dry-run", "diff"} {
		if err := cmd.Flags().Set(flag, "true"); err != nil {
			t.Fatalf("set %s: %v", flag, err)
		}
	}
	if err := runUpgrade(cmd, "v2.0.0"); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}

	var envelope struct {
		Summary string `json:"summary"`
		Data    struct {
			Artifacts mutationReport `json:"artifacts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	artifacts := envelope.Data.Artifacts
	if want := []string{"andurel.toml", "go.mod"}; !slices.Equal(artifacts.FilesUpdated, want) {
		t.Fatalf("files_updated = %v, want %v", artifacts.FilesUpdated, want)
	}
	if want := []string{"internal/old.go"}; !slices.Equal(artifacts.FilesDeleted, want) {
		t.Fatalf("files_deleted = %v, want %v", artifacts.FilesDeleted, want)
	}
	if envelope.Summary != "Would change 3 files for upgrade" {
		t.Fatalf("summary = %q", envelope.Summary)
	}
	for _, want := range []string{"+++ b/andurel.toml", "+++ b/go.mod", `version = "v2.0.0"`} {
		if !strings.Contains(artifacts.Diff, want) {
			t.Fatalf("dry-run diff missing %q:\n%s", want, artifacts.Diff)
		}
	}
}

func TestMutationSummaryPluralizesFiles(t *testing.T) {
	report := mutationReport{Action: "upgrade", DryRun: true, FilesUpdated: []string{"go.mod"}}
	if got := mutationSummary(report); got != "Would change 1 file for upgrade" {
		t.Fatalf("summary = %q", got)
	}
}

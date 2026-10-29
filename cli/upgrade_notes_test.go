package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
)

func runUpgradeNotesTest(t *testing.T, version string, args ...string) (string, error) {
	t.Helper()
	resetCLITestSeams(t)

	rootDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(rootDir, "go.mod"),
		[]byte("module example.com/app\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	lock := layout.NewAndurelLock(version)
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:              "app",
		Inertia:                  "react",
		JavaScriptPackageManager: "pnpm",
	}
	if err := lock.WriteLockFile(rootDir); err != nil {
		t.Fatalf("write andurel.lock: %v", err)
	}
	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"inspect", "upgrade-notes"}, args...))
	err := cmd.Execute()
	return stdout.String(), err
}

func TestUpgradeNotesListsBetaThreeActionsAfterUpgrade(t *testing.T) {
	stdout, err := runUpgradeNotesTest(t, "v2.0.0-beta.3", "--from", "v2.0.0-beta.2", "--json")
	if err != nil {
		t.Fatalf("upgrade-notes: %v", err)
	}

	var envelope struct {
		OK   bool               `json:"ok"`
		Data upgradeNotesReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, stdout)
	}
	if !envelope.OK || envelope.Data.ToVersion != "v2.0.0-beta.3" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}

	ids := make([]string, 0, len(envelope.Data.ManualActions))
	for _, action := range envelope.Data.ManualActions {
		ids = append(ids, action.ID)
		if !strings.Contains(action.Instructions, "Already fixed this yourself?") {
			t.Fatalf("action %s has no equivalence guidance", action.ID)
		}
	}
	want := []string{
		"cors-host-origins-v2.0.0-beta.3",
		"csrf-asset-skip-v2.0.0-beta.3",
		"method-not-allowed-page-v2.0.0-beta.3",
		"request-host-middleware-v2.0.0-beta.3",
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("action IDs = %v, want %v", ids, want)
	}
}

func TestUpgradeNotesHumanOutputNamesActions(t *testing.T) {
	stdout, err := runUpgradeNotesTest(t, "v2.0.0-beta.3", "--from", "v2.0.0-beta.2")
	if err != nil {
		t.Fatalf("upgrade-notes: %v", err)
	}
	for _, want := range []string{
		"Manual actions from v2.0.0-beta.2 to v2.0.0-beta.3:",
		"Render the themed not-found page for 405 responses (method-not-allowed-page-v2.0.0-beta.3)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestUpgradeNotesRejectsMissingOrNonOlderFrom(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--from", "v2.0.0-beta.3"},
		{"--from", "not-a-version"},
	} {
		_, err := runUpgradeNotesTest(t, "v2.0.0-beta.3", args...)
		var cliErr *output.CLIError
		if !errors.As(err, &cliErr) || cliErr.Code != output.CodeUsage {
			t.Fatalf("args %v: err = %v, want usage error", args, err)
		}
	}
}

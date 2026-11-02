package cli

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
)

func TestStructuredUpgradeFailedGitCheckCarriesProvenanceInErrorData(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	lock := layout.NewAndurelLock("v2.0.0-alpha.4")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{ProjectName: "app"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	prepareUpgradeJSONTest(t, root)

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, false, false)
	err := runUpgrade(cmd, "v2.0.0")
	if err == nil {
		t.Fatal("expected the real upgrader git check to fail outside a repository")
	}
	if out.Len() != 0 {
		t.Fatalf("failed structured upgrade emitted a success envelope:\n%s", out.String())
	}

	envelope := output.Fail(err)
	if envelope.OK {
		t.Fatalf("error envelope is ok:\n%#v", envelope)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("error data = %T, want map with report provenance", envelope.Data)
	}
	if _, ok := data["upgrade"].(*upgrade.UpgradeReport); !ok {
		t.Fatalf("error data missing upgrade report: %#v", data)
	}
	commands, ok := data["commands_run"].([]string)
	if !ok || !slices.Equal(commands, []string{"git status --porcelain (failed)"}) {
		t.Fatalf("error commands_run = %#v, want the failed git check", data["commands_run"])
	}
}

func TestStructuredUpgradeUnsupportedErrorKeepsMetadataAndProvenance(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	prepareUpgradeJSONTest(t, root)
	stubLatestAndurelVersion(t, "v2.0.0-alpha.5", nil)

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return &upgradeJSONFake{
			report: &upgrade.UpgradeReport{
				CommandsRun: []string{"git status --porcelain (failed)"},
			},
			err: &upgrade.UnsupportedUpgradeError{
				FromVersion:    "v2.0.0-alpha.3",
				ToVersion:      "v2.0.0-alpha.5",
				RebuildVersion: "v2.0.0-alpha.4",
				LegacyMarkers:  []string{"controllers/", "go.uber.org/fx"},
			},
		}, nil
	}

	var out bytes.Buffer
	cmd := newUpgradeJSONCommand(t, &out, false, false)
	err := runUpgrade(cmd, "v2.0.0-alpha.5")
	if err == nil {
		t.Fatal("expected the unsupported upgrade to fail")
	}
	if out.Len() != 0 {
		t.Fatalf("failed structured upgrade emitted a success envelope:\n%s", out.String())
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error = %T %v, want *output.CLIError", err, err)
	}
	if cliErr.Code != output.CodeUnsupportedUpgrade || cliErr.ExitCode != output.ExitConfig {
		t.Fatalf("typed error = %#v", cliErr)
	}
	if !strings.Contains(cliErr.Hint, "domains/") {
		t.Fatalf("typed hint = %q", cliErr.Hint)
	}

	data, ok := cliErr.Data.(map[string]any)
	if !ok {
		t.Fatalf("error data = %T, want map", cliErr.Data)
	}
	for key, want := range map[string]string{
		"from":            "v2.0.0-alpha.3",
		"to":              "v2.0.0-alpha.5",
		"rebuild_version": "v2.0.0-alpha.4",
	} {
		if got, ok := data[key].(string); !ok || got != want {
			t.Fatalf("error data %s = %#v, want %q", key, data[key], want)
		}
	}
	if got, ok := data["legacy_markers"].([]string); !ok ||
		!slices.Equal(got, []string{"controllers/", "go.uber.org/fx"}) {
		t.Fatalf("legacy_markers = %#v", data["legacy_markers"])
	}
	if got, ok := data["commands_run"].([]string); !ok ||
		!slices.Equal(got, []string{"git status --porcelain (failed)"}) {
		t.Fatalf("commands_run = %#v", data["commands_run"])
	}
	if _, ok := data["upgrade"].(*upgrade.UpgradeReport); !ok {
		t.Fatalf("error data missing upgrade report: %#v", data)
	}
}

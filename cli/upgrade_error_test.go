package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
)

func TestRunUpgradeMapsUnsupportedUpgradeToStructuredError(t *testing.T) {
	resetCLITestSeams(t)
	stubLatestAndurelVersion(t, "v2.0.0-alpha.5", nil)

	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	newUpgraderFunc = func(string, upgrade.UpgradeOptions) (cliUpgrader, error) {
		return fakeUpgrader{err: &upgrade.UnsupportedUpgradeError{
			FromVersion:    "v2.0.0-alpha.3",
			ToVersion:      "v2.0.0-alpha.5",
			RebuildVersion: "v2.0.0-alpha.4",
			LegacyMarkers:  []string{"controllers/", "go.uber.org/fx"},
		}}, nil
	}

	cmd := newStructuredTestCommand(&bytes.Buffer{})
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("diff", false, "")

	err := runUpgrade(cmd, "v2.0.0-alpha.5")
	if err == nil {
		t.Fatal("expected the unsupported upgrade to fail")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error = %T %v, want *output.CLIError", err, err)
	}
	if cliErr.Code != output.CodeUnsupportedUpgrade {
		t.Fatalf("code = %q, want %q", cliErr.Code, output.CodeUnsupportedUpgrade)
	}
	if cliErr.ExitCode != output.ExitConfig {
		t.Fatalf("exit code = %d, want %d", cliErr.ExitCode, output.ExitConfig)
	}
	if !strings.Contains(cliErr.Hint, "domains/") ||
		!strings.Contains(cliErr.Hint, "patterns show composition") {
		t.Fatalf("hint = %q", cliErr.Hint)
	}

	envelope := output.Fail(err)
	if envelope.Code != output.CodeUnsupportedUpgrade || envelope.ExitCode != output.ExitConfig {
		t.Fatalf("structured envelope = %#v", envelope)
	}
	encoded, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, want := range []string{
		`"code":"unsupported_upgrade"`,
		`"from":"v2.0.0-alpha.3"`,
		`"to":"v2.0.0-alpha.5"`,
		`"rebuild_version":"v2.0.0-alpha.4"`,
		`"legacy_markers":["controllers/","go.uber.org/fx"]`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("structured envelope missing %s:\n%s", want, encoded)
		}
	}

	var rendered bytes.Buffer
	cmd.SetErr(&rendered)
	if renderErr := output.RenderError(cmd, err); renderErr != nil {
		t.Fatalf("render error: %v", renderErr)
	}
	if !strings.Contains(rendered.String(), `"code": "unsupported_upgrade"`) {
		t.Fatalf("JSON error output missing typed code:\n%s", rendered.String())
	}
}

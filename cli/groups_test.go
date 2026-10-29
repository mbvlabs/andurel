package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func TestInspectUnknownSubcommandIsUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"inspect", "definitely-not-a-command"},
		{"inspect", "definitely-not-a-command", "--json"},
	} {
		result := runCLITest(t, args...)
		if result.err == nil {
			t.Fatalf("%v: expected a usage error", args)
		}
		envelope := output.Fail(result.err)
		if envelope.Code != output.CodeUsage || envelope.ExitCode != output.ExitUsage {
			t.Fatalf("%v: envelope = %#v", args, envelope)
		}
		if !strings.Contains(envelope.Error, "unknown subcommand") {
			t.Fatalf("%v: error = %q", args, envelope.Error)
		}
		if !strings.Contains(envelope.Hint, "pages") {
			t.Fatalf("%v: expected a suggestion near pages, hint = %q", args, envelope.Hint)
		}
	}
}

func TestUnknownSubcommandRendersJSONErrorEnvelope(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"inspect", "definitely-not-a-command", "--json"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected a usage error")
	}
	if renderErr := output.RenderError(root, err); renderErr != nil {
		t.Fatalf("render error: %v", renderErr)
	}

	var envelope output.ErrorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode JSON error: %v\nstderr:\n%s", err, stderr.String())
	}
	if envelope.OK || envelope.Code != output.CodeUsage ||
		envelope.ExitCode != output.ExitUsage {
		t.Fatalf("JSON error = %#v", envelope)
	}
	if !strings.Contains(envelope.Error, "unknown subcommand") {
		t.Fatalf("error = %q", envelope.Error)
	}
	if !strings.Contains(envelope.Hint, "pages") {
		t.Fatalf("hint = %q", envelope.Hint)
	}
}

func TestBareGroupsShowHelp(t *testing.T) {
	for _, args := range [][]string{
		{"inspect"},
		{"generate"},
		{"sync"},
		{"patterns"},
		{"db"},
		{"db", "migrate"},
		{"tool"},
		{"packages"},
		{"skill"},
	} {
		result := runCLITest(t, args...)
		if result.err != nil {
			t.Fatalf("%v: %v\nstderr:\n%s", args, result.err, result.stderr)
		}
		if !strings.Contains(result.stdout, "Usage:") ||
			!strings.Contains(result.stdout, "Commands:") {
			t.Fatalf("%v: bare group did not render help:\n%s", args, result.stdout)
		}
	}
}

func TestUnknownSubcommandsRejectedAcrossGroups(t *testing.T) {
	for _, group := range [][]string{
		{"generate"},
		{"sync"},
		{"inspect"},
		{"inspect", "project"},
		{"patterns"},
		{"db"},
		{"db", "migrate"},
		{"tool"},
		{"packages"},
		{"skill"},
	} {
		args := append(append([]string(nil), group...), "definitely-not-a-command")
		result := runCLITest(t, args...)
		if result.err == nil {
			t.Fatalf("%v: expected a usage error", args)
		}
		envelope := output.Fail(result.err)
		if envelope.Code != output.CodeUsage || envelope.ExitCode != output.ExitUsage {
			t.Fatalf("%v: envelope = %#v", args, envelope)
		}
	}
}

func TestInspectHelpStillSucceeds(t *testing.T) {
	for _, args := range [][]string{
		{"inspect", "--help"},
		{"inspect", "pages", "--help"},
	} {
		result := runCLITest(t, args...)
		if result.err != nil {
			t.Fatalf("%v: %v\nstderr:\n%s", args, result.err, result.stderr)
		}
	}
}

func TestValidNestedCommandsStillRun(t *testing.T) {
	for _, args := range [][]string{
		{"inspect", "project", "info", "--json"},
		{"patterns", "list", "--json"},
		{"patterns", "show", "domain", "--json"},
		{"skill", "show", "--json"},
		{"tool", "list", "--json"},
	} {
		result := runCLITest(t, args...)
		if result.err != nil {
			t.Fatalf("%v: %v\nstderr:\n%s", args, result.err, result.stderr)
		}
		if !strings.Contains(result.stdout, `"ok": true`) {
			t.Fatalf("%v: missing JSON envelope:\n%s", args, result.stdout)
		}
	}
}

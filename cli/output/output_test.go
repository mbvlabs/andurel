package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseOptionsSelectsAgentMode(t *testing.T) {
	cmd := &cobra.Command{Use: "andurel"}
	RegisterPersistentFlags(cmd)
	if err := cmd.PersistentFlags().Set("agent", "true"); err != nil {
		t.Fatalf("set --agent: %v", err)
	}
	if err := cmd.PersistentFlags().Set("jq", ".data"); err != nil {
		t.Fatalf("set --jq: %v", err)
	}

	opts, err := ParseOptions(cmd)
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}
	if opts.Mode != ModeAgent {
		t.Fatalf("expected agent mode, got %s", opts.Mode)
	}
	if opts.JQ != ".data" {
		t.Fatalf("expected jq expression to be preserved, got %q", opts.JQ)
	}
}

func TestParseOptionsRejectsMultipleOutputModes(t *testing.T) {
	cmd := &cobra.Command{Use: "andurel"}
	RegisterPersistentFlags(cmd)
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set --json: %v", err)
	}
	if err := cmd.PersistentFlags().Set("md", "true"); err != nil {
		t.Fatalf("set --md: %v", err)
	}

	_, err := ParseOptions(cmd)
	if err == nil {
		t.Fatalf("expected conflict error")
	}

	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != CodeOutputMode {
		t.Fatalf("expected code %q, got %q", CodeOutputMode, cliErr.Code)
	}
}

func TestOKRendersJSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "andurel"}
	RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set --json: %v", err)
	}

	err := OK(cmd, map[string]string{"id": "post"}, "Generated post", Breadcrumb{Command: "andurel run"})
	if err != nil {
		t.Fatalf("OK returned error: %v", err)
	}

	var envelope Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK || envelope.Summary != "Generated post" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if len(envelope.Breadcrumbs) != 1 || envelope.Breadcrumbs[0].Command != "andurel run" {
		t.Fatalf("unexpected breadcrumbs: %#v", envelope.Breadcrumbs)
	}
}

func TestFailPreservesTypedErrorFields(t *testing.T) {
	err := NewError("project_not_found", "not in an Andurel project", ExitProject, "Run this from a project root.")

	envelope := Fail(err)
	if envelope.OK {
		t.Fatalf("error envelope should not be ok")
	}
	if envelope.Code != "project_not_found" || envelope.ExitCode != ExitProject {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if !strings.Contains(envelope.Hint, "project root") {
		t.Fatalf("expected hint to be preserved, got %q", envelope.Hint)
	}
}

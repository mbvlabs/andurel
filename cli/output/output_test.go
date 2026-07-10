package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

func TestParseOptionsDefaultsProjectionToJSONAndReadsBooleans(t *testing.T) {
	cmd := &cobra.Command{Use: "andurel"}
	RegisterPersistentFlags(cmd)
	if err := cmd.PersistentFlags().Set("jq", ".data.id"); err != nil {
		t.Fatalf("set --jq: %v", err)
	}
	if err := cmd.PersistentFlags().Set("quiet", "true"); err != nil {
		t.Fatalf("set --quiet: %v", err)
	}
	if err := cmd.PersistentFlags().Set("verbose", "true"); err != nil {
		t.Fatalf("set --verbose: %v", err)
	}

	opts, err := ParseOptions(cmd)
	if err != nil {
		t.Fatalf("ParseOptions: %v", err)
	}
	if opts.Mode != ModeJSON || opts.JQ != ".data.id" || !opts.Quiet || opts.IDsOnly || opts.Count || !opts.Verbose {
		t.Fatalf("unexpected options: %#v", opts)
	}
	if !UsesStructuredOutput(opts) || !SuppressesHumanOutput(opts) {
		t.Fatalf("expected jq-selected JSON mode to be structured and suppress human output")
	}
	if SuppressesHumanOutput(Options{Mode: ModeHuman}) {
		t.Fatalf("plain human output should not be suppressed")
	}
	if !SuppressesHumanOutput(Options{Mode: ModeHuman, Quiet: true}) {
		t.Fatalf("quiet human output should be suppressed")
	}
}

func TestOKRendersHumanMarkdownQuietAndJQ(t *testing.T) {
	tests := []struct {
		name     string
		flags    map[string]string
		want     []string
		notWant  []string
		validate func(t *testing.T, out string)
	}{
		{
			name:  "human",
			flags: map[string]string{},
			want:  []string{"Generated post", "Next steps:", "andurel route:list - inspect routes"},
		},
		{
			name:  "markdown",
			flags: map[string]string{"md": "true"},
			want:  []string{"Generated post", "Next steps:", "- `andurel route:list - inspect routes`"},
		},
		{
			name:    "quiet",
			flags:   map[string]string{"quiet": "true"},
			notWant: []string{"Generated post", "Next steps:"},
		},
		{
			name:  "jq",
			flags: map[string]string{"jq": ".id"},
			validate: func(t *testing.T, out string) {
				t.Helper()
				var selected string
				if err := json.Unmarshal([]byte(out), &selected); err != nil {
					t.Fatalf("decode jq output: %v\n%s", err, out)
				}
				if selected != "post" {
					t.Fatalf("jq selected value = %#v, want post", selected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: "andurel"}
			RegisterPersistentFlags(cmd)
			cmd.SetOut(&out)
			for flag, value := range tt.flags {
				if err := cmd.PersistentFlags().Set(flag, value); err != nil {
					t.Fatalf("set --%s: %v", flag, err)
				}
			}

			err := OK(
				cmd,
				map[string]any{"id": "post", "count": 2},
				"Generated post",
				Breadcrumb{Command: "andurel route:list", Description: "inspect routes"},
			)
			if err != nil {
				t.Fatalf("OK: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("output missing %q:\n%s", want, out.String())
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(out.String(), notWant) {
					t.Fatalf("output contained %q:\n%s", notWant, out.String())
				}
			}
			if tt.validate != nil {
				tt.validate(t, out.String())
			}
		})
	}
}

func TestApplyJQAndLookupField(t *testing.T) {
	type nested struct {
		Name   string `json:"name"`
		Amount int
		hidden string
	}
	data := struct {
		Item *nested `json:"item"`
	}{
		Item: &nested{Name: "invoice", Amount: 42, hidden: "secret"},
	}

	got, err := applyJQ(data, ".item.name")
	if err != nil {
		t.Fatalf("applyJQ json tag path: %v", err)
	}
	if got != "invoice" {
		t.Fatalf("applyJQ json tag path = %#v", got)
	}

	got, err = applyJQ(data, ".item.Amount")
	if err != nil {
		t.Fatalf("applyJQ field name path: %v", err)
	}
	if got != float64(42) {
		t.Fatalf("applyJQ field name path = %#v", got)
	}

	if _, err := applyJQ(data, "data.item"); err == nil {
		t.Fatalf("expected unsupported jq expression error")
	}
	if _, err := applyJQ(data, ".missing"); err == nil {
		t.Fatalf("expected missing jq path error")
	}
}

func TestFailClassifiesCommonErrorsAndExitCode(t *testing.T) {
	tests := []struct {
		err      error
		code     string
		exitCode int
	}{
		{fmt.Errorf("go.mod could not be found"), CodeProjectNotFound, ExitProject},
		{fmt.Errorf("bin/air not found"), CodeMissingTool, ExitDependency},
		{fmt.Errorf("unknown extension: queue"), CodeInvalidExtension, ExitUsage},
		{fmt.Errorf("invalid inertia adapter svelte"), CodeInvalidInertiaAdapter, ExitUsage},
		{fmt.Errorf("delete requires --force"), CodeUnsafeAction, ExitUnsafe},
		{fmt.Errorf("generation failed for model"), CodeGenerationFailed, ExitGeneration},
		{fmt.Errorf("external command failed"), CodeExternalCommandFailed, ExitExternal},
		{fmt.Errorf("andurel.lock is invalid"), CodeConfigError, ExitConfig},
		{fmt.Errorf("ambiguous resource name"), CodeAmbiguousInput, ExitAmbiguous},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			envelope := Fail(tt.err)
			if envelope.Code != tt.code || envelope.ExitCode != tt.exitCode || envelope.Hint == "" {
				t.Fatalf("Fail(%v) = %#v", tt.err, envelope)
			}
			if ExitCode(tt.err) != tt.exitCode {
				t.Fatalf("ExitCode(%v) = %d, want %d", tt.err, ExitCode(tt.err), tt.exitCode)
			}
		})
	}

	generic := Fail(errors.New("plain failure"))
	if generic.Code != CodeError || generic.ExitCode != ExitUsage {
		t.Fatalf("unexpected generic envelope: %#v", generic)
	}
	if unknown := Fail(nil); unknown.Error != "unknown error" || unknown.ExitCode != ExitUsage {
		t.Fatalf("unexpected nil envelope: %#v", unknown)
	}
}

func TestRenderErrorModes(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{name: "human", want: "Error: not in an Andurel project\nHint:"},
		{name: "markdown", flag: "md", want: "**Error:** not in an Andurel project"},
		{name: "json", flag: "json", want: `"code": "project_not_found"`},
		{name: "agent", flag: "agent", want: `"exit_code": 2`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cmd := &cobra.Command{Use: "andurel"}
			RegisterPersistentFlags(cmd)
			cmd.SetErr(&stderr)
			if tt.flag != "" {
				if err := cmd.PersistentFlags().Set(tt.flag, "true"); err != nil {
					t.Fatalf("set --%s: %v", tt.flag, err)
				}
			}

			err := NewError(CodeProjectNotFound, "not in an Andurel project", ExitProject, "Run from a project root.")
			if renderErr := RenderError(cmd, err); renderErr != nil {
				t.Fatalf("RenderError: %v", renderErr)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr missing %q:\n%s", tt.want, stderr.String())
			}
		})
	}
}

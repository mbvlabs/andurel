package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type catalogCapabilityRecord struct {
	Path             string   `json:"path"`
	SupportsJSON     bool     `json:"supports_json"`
	SupportsJQ       bool     `json:"supports_jq"`
	SupportsIDsOnly  bool     `json:"supports_ids_only"`
	SupportsCount    bool     `json:"supports_count"`
	SupportsDryRun   bool     `json:"supports_dry_run"`
	InteractiveFlags []string `json:"interactive_flags"`
	Examples         []string `json:"examples"`
}

func catalogRecordsByPath(t *testing.T) map[string]catalogCapabilityRecord {
	t.Helper()
	result := runCLITest(t, "commands", "--json")
	if result.err != nil {
		t.Fatalf("commands --json: %v\nstderr:\n%s", result.err, result.stderr)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Catalog []catalogCapabilityRecord `json:"catalog"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
		t.Fatalf("decode commands catalog: %v\n%s", err, result.stdout)
	}
	if !envelope.OK {
		t.Fatalf("commands catalog not ok:\n%s", result.stdout)
	}
	records := make(map[string]catalogCapabilityRecord, len(envelope.Data.Catalog))
	for _, record := range envelope.Data.Catalog {
		records[record.Path] = record
	}
	return records
}

func TestCommandsCatalogSeparatesJSONAndProjectionSupport(t *testing.T) {
	records := catalogRecordsByPath(t)

	for _, path := range []string{
		"andurel build",
		"andurel inspect config",
		"andurel inspect pages",
		"andurel inspect process",
		"andurel inspect urls",
		"andurel inspect",
	} {
		record, ok := records[path]
		if !ok {
			t.Fatalf("missing catalog record %q", path)
		}
		if !record.SupportsJSON {
			t.Errorf("%s: supports_json = false, want true", path)
		}
	}

	build := records["andurel build"]
	if build.SupportsJQ {
		t.Errorf("andurel build: supports_jq = true, want false")
	}
	config := records["andurel inspect config"]
	if !config.SupportsJQ {
		t.Errorf("andurel inspect config: supports_jq = false, want true")
	}

	for _, path := range []string{"andurel inspect pages", "andurel inspect process", "andurel inspect urls"} {
		if !records[path].SupportsJQ {
			t.Errorf("%s: supports_jq = false, want true", path)
		}
	}

	for _, path := range []string{
		"andurel run",
		"andurel fmt",
		"andurel console",
		"andurel db console",
		"andurel db migrate status",
		"andurel generate migration",
	} {
		if records[path].SupportsJSON {
			t.Errorf("%s: supports_json = true, want false", path)
		}
		if records[path].SupportsJQ {
			t.Errorf("%s: supports_jq = true, want false", path)
		}
	}
}

func TestInspectConfigJQKeysWorks(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n")
	writeTestFile(t, root, "config/app.go", `package config

import "github.com/mbvlabs/andurel/pkg/env"

func New() {
	e := env.New()
	_ = e.String("ENVIRONMENT", "development")
	_ = e.RequiredString("HOST_PRIMARY")
}
`)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findGoModRoot = originalFindGoModRoot })

	var stdout bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"inspect", "config", "--jq", ".keys"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect config --jq .keys: %v\n%s", err, stdout.String())
	}

	var keys []configKeyInfo
	if err := json.Unmarshal(stdout.Bytes(), &keys); err != nil {
		t.Fatalf("decode projected keys: %v\n%s", err, stdout.String())
	}
	if len(keys) != 2 || keys[0].Key != "ENVIRONMENT" || keys[1].Key != "HOST_PRIMARY" {
		t.Fatalf("projected keys = %#v", keys)
	}
	if strings.Contains(stdout.String(), `"ok"`) {
		t.Fatalf("projection leaked envelope:\n%s", stdout.String())
	}
}

func TestCommandsCheckRejectsContradictoryCapabilityMetadata(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	build, _, err := root.Find([]string{"build"})
	if err != nil {
		t.Fatalf("find build: %v", err)
	}
	meta, ok := metaFor(build)
	if !ok {
		t.Fatal("build has no metadata")
	}
	meta.SupportsJSON = false
	commandMetaByCmd[build] = meta

	report := checkCommandMetadata(root)
	if report.OK {
		t.Fatal("expected a contradictory supports_json registration to fail the check")
	}
	if !failureContains(report.Failures, "andurel build", "supports_json") {
		t.Fatalf("missing capability failure in %#v", report.Failures)
	}
}

func TestCommandsCheckRejectsMissingCapabilityRegistration(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	build, _, err := root.Find([]string{"build"})
	if err != nil {
		t.Fatalf("find build: %v", err)
	}
	delete(build.Annotations, capabilityAnnotationJSON)

	report := checkCommandMetadata(root)
	if report.OK {
		t.Fatal("expected a missing JSON capability registration to fail the check")
	}
	if !failureContains(report.Failures, "andurel build", "capability") {
		t.Fatalf("missing capability failure in %#v", report.Failures)
	}
}

func TestCommandsCheckRejectsAdvertisedUnsupportedOutput(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    []string
		example string
		field   string
	}{
		{
			name:    "json on a passthrough command",
			path:    []string{"fmt"},
			example: "andurel fmt --json",
			field:   "--json",
		},
		{
			name:    "jq on a JSON-only command",
			path:    []string{"build"},
			example: "andurel build --jq .artifacts",
			field:   "--jq",
		},
		{
			name:    "dry-run without the flag",
			path:    []string{"db", "nuke"},
			example: "andurel db nuke --dry-run",
			field:   "--dry-run",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := NewRootCommand("test", "test-date")
			cmd, _, err := root.Find(test.path)
			if err != nil {
				t.Fatalf("find %v: %v", test.path, err)
			}
			meta, ok := metaFor(cmd)
			if !ok {
				t.Fatalf("%s has no metadata", cmd.CommandPath())
			}
			meta.Examples = append(append([]string(nil), meta.Examples...), test.example)
			commandMetaByCmd[cmd] = meta

			report := checkCommandMetadata(root)
			if report.OK {
				t.Fatalf("expected advertised %s example to fail the check", test.field)
			}
			if !failureContains(report.Failures, cmd.CommandPath(), test.field) {
				t.Fatalf("missing %s failure in %#v", test.field, report.Failures)
			}
		})
	}
}

func TestCommandsCheckRejectsProjectionOnUnsupportedCommand(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	err := applyProjectionContracts(root, []projectionSupport{
		{path: "fmt", jq: true},
	})
	if err == nil {
		t.Fatal("expected a projection contract for a structured-output-unsupported command to fail")
	}
	if !strings.Contains(err.Error(), "fmt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnsupportedStructuredOutputRemainsUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"db", "console", "--json"},
		{"db", "migrate", "status", "--json"},
		{"generate", "migration", "probe", "--json"},
		{"tool", "set-version", "templ", "1.0.0", "--json"},
		{"run", "--json"},
		{"fmt", "--json"},
	} {
		result := runCLITest(t, args...)
		if result.err == nil {
			t.Fatalf("%v: expected a usage error", args)
		}
		envelope := output.Fail(result.err)
		if envelope.Code != output.CodeUsage || envelope.ExitCode != output.ExitUsage {
			t.Fatalf("%v: envelope = %#v", args, envelope)
		}
		if !strings.Contains(envelope.Error, "does not support") {
			t.Fatalf("%v: error = %q", args, envelope.Error)
		}
	}
}

func TestEveryCommandHasExplicitCapabilityRegistration(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	for _, cmd := range walkPublicCommands(root) {
		if cmd.Annotations[capabilityAnnotationJSON] == "" {
			t.Errorf("%s: missing %s annotation", cmd.CommandPath(), capabilityAnnotationJSON)
		}
	}
}

func failureContains(failures []string, parts ...string) bool {
	for _, failure := range failures {
		matched := true
		for _, part := range parts {
			if !strings.Contains(failure, part) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func TestStructuredOutputRegistryMatchesRejectCallSites(t *testing.T) {
	callPattern := regexp.MustCompile(`rejectStructuredOutput\(cmd, "([^"]+)"\)`)
	actual := map[string]bool{}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob cli sources: %v", err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, match := range callPattern.FindAllStringSubmatch(string(body), -1) {
			actual[match[1]] = true
		}
	}

	registered := map[string]bool{}
	for _, path := range structuredOutputUnsupportedCommands {
		registered[path] = true
	}
	for path := range actual {
		if !registered[path] {
			t.Errorf("%s calls rejectStructuredOutput but is not in structuredOutputUnsupportedCommands", path)
		}
	}
	// andurel console and andurel db console share one constructor and one
	// reject action string ("andurel console").
	sharedRejectActions := map[string]string{
		"andurel db console": "andurel console",
	}
	for path := range registered {
		if actual[path] {
			continue
		}
		if shared, ok := sharedRejectActions[path]; ok && actual[shared] {
			continue
		}
		t.Errorf("%s is registered as structured-output unsupported but has no rejectStructuredOutput call", path)
	}
}

func TestRunCommandsCheckFailsOnCapabilityDrift(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	build, _, err := root.Find([]string{"build"})
	if err != nil {
		t.Fatalf("find build: %v", err)
	}
	meta, ok := metaFor(build)
	if !ok {
		t.Fatal("build has no metadata")
	}
	meta.Examples = append(append([]string(nil), meta.Examples...), "andurel build --dry-run")
	commandMetaByCmd[build] = meta

	cmd := &cobra.Command{Use: "commands"}
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&bytes.Buffer{})
	err = runCommandsCheck(cmd, root)
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeUsage {
		t.Fatalf("error = %T %[1]v", err)
	}
	failures, ok := cliErr.Data.(commandMetaCheckReport)
	if !ok || failures.OK || len(failures.Failures) == 0 {
		t.Fatalf("report = %#v", cliErr.Data)
	}
	if !failureContains(failures.Failures, "andurel build", "--dry-run") {
		t.Fatalf("failures = %#v", failures.Failures)
	}
}

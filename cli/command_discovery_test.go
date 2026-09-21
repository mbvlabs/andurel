package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

type discoverySummary struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type flagSummary struct {
	Name string `json:"name"`
}

func TestCommandsJSONDiscovery(t *testing.T) {
	result := runCLITest(t, "commands", "--json")
	if result.err != nil {
		t.Fatalf("commands --json returned error: %v\nstderr:\n%s", result.err, result.stderr)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Name        string             `json:"name"`
			Path        string             `json:"path"`
			Subcommands []discoverySummary `json:"subcommands"`
			Commands    []discoverySummary `json:"commands"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
		t.Fatalf("decode commands output: %v\nstdout:\n%s", err, result.stdout)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope: %#v", envelope)
	}
	if envelope.Data.Name != "andurel" || envelope.Data.Path != "andurel" {
		t.Fatalf("unexpected root discovery: %#v", envelope.Data)
	}
	if !discoveryContains(envelope.Data.Subcommands, "generate", "andurel generate") {
		t.Fatalf("expected generate subcommand in discovery: %#v", envelope.Data.Subcommands)
	}
	if !discoveryContains(envelope.Data.Commands, "commands", "andurel commands") {
		t.Fatalf("expected commands command in full tree: %#v", envelope.Data.Commands)
	}
}

func TestCommandsCatalogAndCheck(t *testing.T) {
	jsonResult := runCLITest(t, "commands", "--json")
	if jsonResult.err != nil {
		t.Fatalf("commands --json returned error: %v\nstderr:\n%s", jsonResult.err, jsonResult.stderr)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Catalog []struct {
				Path      string   `json:"path"`
				Summary   string   `json:"summary"`
				WhenToUse []string `json:"when_to_use"`
			} `json:"catalog"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonResult.stdout), &envelope); err != nil {
		t.Fatalf("decode catalog: %v\nstdout:\n%s", err, jsonResult.stdout)
	}
	if !envelope.OK || len(envelope.Data.Catalog) == 0 {
		t.Fatalf("expected catalog records: %#v", envelope.Data)
	}

	markdownResult := runCLITest(t, "commands", "--markdown")
	if markdownResult.err != nil {
		t.Fatalf(
			"commands --markdown returned error: %v\nstderr:\n%s",
			markdownResult.err,
			markdownResult.stderr,
		)
	}
	if !strings.Contains(markdownResult.stdout, "| Route | Summary |") {
		t.Fatalf("expected markdown table:\n%s", markdownResult.stdout)
	}

	checkResult := runCLITest(t, "commands", "--check", "--json")
	if checkResult.err != nil {
		t.Fatalf(
			"commands --check --json returned error: %v\nstderr:\n%s\nstdout:\n%s",
			checkResult.err,
			checkResult.stderr,
			checkResult.stdout,
		)
	}
}

func TestAgentHelpDiscovery(t *testing.T) {
	result := runCLITest(t, "--agent", "--help")
	if result.err != nil {
		t.Fatalf("--agent --help returned error: %v\nstderr:\n%s", result.err, result.stderr)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Name        string             `json:"name"`
			Path        string             `json:"path"`
			LocalFlags  []flagSummary      `json:"local_flags"`
			Subcommands []discoverySummary `json:"subcommands"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
		t.Fatalf("decode agent help: %v\nstdout:\n%s", err, result.stdout)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope: %#v", envelope)
	}
	if envelope.Data.Name != "andurel" || envelope.Data.Path != "andurel" {
		t.Fatalf("unexpected help data: %#v", envelope.Data)
	}
	if !flagDiscoveryContains(envelope.Data.LocalFlags, "agent") {
		t.Fatalf("expected root output flags in help: %#v", envelope.Data.LocalFlags)
	}
	if !discoveryContains(envelope.Data.Subcommands, "commands", "andurel commands") {
		t.Fatalf("expected commands subcommand in help: %#v", envelope.Data.Subcommands)
	}
}

func TestGenerateAgentHelpDiscovery(t *testing.T) {
	result := runCLITest(t, "generate", "--agent", "--help")
	if result.err != nil {
		t.Fatalf(
			"generate --agent --help returned error: %v\nstderr:\n%s",
			result.err,
			result.stderr,
		)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Name           string             `json:"name"`
			Path           string             `json:"path"`
			Category       string             `json:"category"`
			AgentNotes     string             `json:"agent_notes"`
			InheritedFlags []flagSummary      `json:"inherited_flags"`
			Subcommands    []discoverySummary `json:"subcommands"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
		t.Fatalf("decode generate agent help: %v\nstdout:\n%s", err, result.stdout)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope: %#v", envelope)
	}
	if envelope.Data.Name != "generate" || envelope.Data.Path != "andurel generate" {
		t.Fatalf("unexpected generate help data: %#v", envelope.Data)
	}
	if envelope.Data.Category != "generate" || envelope.Data.AgentNotes == "" {
		t.Fatalf("expected generate agent metadata: %#v", envelope.Data)
	}
	if !flagDiscoveryContains(envelope.Data.InheritedFlags, "json") {
		t.Fatalf("expected inherited output flags: %#v", envelope.Data.InheritedFlags)
	}
	if !discoveryContains(envelope.Data.Subcommands, "scaffold", "andurel generate scaffold") {
		t.Fatalf("expected scaffold subcommand in help: %#v", envelope.Data.Subcommands)
	}
}

func TestNewInertiaAndSQLCCommandsExposeAgentNotes(t *testing.T) {
	for _, path := range []string{
		"new",
		"generate controller",
		"generate scaffold",
		"generate query",
		"generate migration",
	} {
		root := NewRootCommand("test", "test-date")
		cmd, _, err := root.Find(strings.Fields(path))
		if err != nil {
			t.Fatalf("find %s: %v", path, err)
		}
		if cmd.Annotations[agentCategoryAnnotation] == "" ||
			cmd.Annotations[agentNotesAnnotation] == "" {
			t.Fatalf("%s is missing agent discovery metadata", path)
		}
	}
}

func discoveryContains(commands []discoverySummary, name, path string) bool {
	for _, command := range commands {
		if command.Name == name && command.Path == path {
			return true
		}
	}
	return false
}

func flagDiscoveryContains(flags []flagSummary, name string) bool {
	for _, flag := range flags {
		if flag.Name == name {
			return true
		}
	}
	return false
}

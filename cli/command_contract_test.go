package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type discoverySummary struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type flagSummary struct {
	Name string `json:"name"`
}

func TestRootCommandPublicSurface(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	expected := []commandContract{
		{name: "build"},
		{name: "commands"},
		{name: "config"},
		{name: "console", aliases: []string{"c"}},
		{name: "controllers"},
		{name: "database", aliases: []string{"d", "db"}},
		{name: "doctor", aliases: []string{"doc"}},
		{name: "extension", aliases: []string{"extensions", "ext", "e"}},
		{name: "fmt", aliases: []string{"f"}},
		{name: "generate", aliases: []string{"g"}},
		{name: "jobs"},
		{name: "migrations"},
		{name: "models"},
		{name: "new", aliases: []string{"n"}},
		{name: "project"},
		{name: "routes"},
		{name: "run", aliases: []string{"r"}},
		{name: "skill"},
		{name: "tool", aliases: []string{"tools", "t"}},
		{name: "upgrade", aliases: []string{"up"}},
		{name: "views"},
	}

	assertCommandSurface(t, rootCmd, expected)
}

func TestGenerateCommandPublicSurface(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")
	generateCmd := mustFindCommand(t, rootCmd, "generate")

	expected := []commandContract{
		{name: "controller", aliases: []string{"c"}},
		{name: "email", aliases: []string{"e"}},
		{name: "factories"},
		{name: "factory"},
		{name: "job", aliases: []string{"j"}},
		{name: "model", aliases: []string{"m"}},
		{name: "routes"},
		{name: "scaffold", aliases: []string{"s"}},
		{name: "view", aliases: []string{"v"}},
	}

	assertCommandSurface(t, generateCmd, expected)
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
		t.Fatalf("generate --agent --help returned error: %v\nstderr:\n%s", result.err, result.stderr)
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
	if envelope.Data.Category != "generation" || envelope.Data.AgentNotes == "" {
		t.Fatalf("expected generate agent metadata: %#v", envelope.Data)
	}
	if !flagDiscoveryContains(envelope.Data.InheritedFlags, "json") {
		t.Fatalf("expected inherited output flags: %#v", envelope.Data.InheritedFlags)
	}
	if !discoveryContains(envelope.Data.Subcommands, "scaffold", "andurel generate scaffold") {
		t.Fatalf("expected scaffold subcommand in help: %#v", envelope.Data.Subcommands)
	}
}

func TestCommandFlagsContract(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		path  string
		flags []string
	}{
		{path: "new", flags: []string{"extensions", "inertia", "dry-run", "diff"}},
		{path: "generate model", flags: []string{"skip-factory", "table-name", "update", "yes", "primary-key", "dry-run", "diff"}},
		{path: "generate factory", flags: []string{"check", "sync", "diff"}},
		{path: "generate factories", flags: []string{"check", "sync", "diff"}},
		{path: "generate controller", flags: []string{"inertia", "model-name", "dry-run", "diff"}},
		{path: "generate scaffold", flags: []string{"skip-factory", "table-name", "primary-key", "inertia", "dry-run", "diff"}},
		{path: "generate job", flags: []string{"queue", "dry-run", "diff"}},
		{path: "generate email", flags: []string{"dry-run", "diff"}},
		{path: "extension add", flags: []string{"dry-run", "diff"}},
		{path: "extension list", flags: []string{"available"}},
		{path: "fmt", flags: []string{"check", "skip-templ", "skip-go"}},
		{path: "database drop", flags: []string{"force"}},
		{path: "database nuke", flags: []string{"force"}},
		{path: "database seed", flags: []string{"list"}},
		{path: "database rebuild", flags: []string{"force", "skip-seed", "seed"}},
		{path: "build", flags: []string{"version"}},
		{path: "doctor", flags: []string{"verbose"}},
		{path: "upgrade", flags: []string{"dry-run", "diff"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			cmd := mustFindCommand(t, rootCmd, tt.path)
			for _, flag := range tt.flags {
				if cmd.Flags().Lookup(flag) == nil {
					t.Fatalf("%q missing --%s flag", tt.path, flag)
				}
			}
		})
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

func TestRootCommandPersistentOutputFlags(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	for _, flag := range []string{"json", "agent", "md", "quiet", "jq", "ids-only", "count", "verbose"} {
		if rootCmd.PersistentFlags().Lookup(flag) == nil {
			t.Fatalf("root command missing persistent --%s flag", flag)
		}
	}
}

type commandContract struct {
	name    string
	aliases []string
}

func assertCommandSurface(t *testing.T, parent *cobra.Command, expected []commandContract) {
	t.Helper()

	available := availableCommands(parent)
	if len(available) != len(expected) {
		t.Fatalf("expected commands %v, got %v", commandContractNames(expected), commandNames(available))
	}

	for i, want := range expected {
		got := available[i]
		if got.Name() != want.name {
			t.Fatalf("command %d: expected %q, got %q; all commands: %v", i, want.name, got.Name(), commandNames(available))
		}
		if !slices.Equal(got.Aliases, want.aliases) {
			t.Fatalf("%s aliases: expected %v, got %v", want.name, want.aliases, got.Aliases)
		}
	}
}

func availableCommands(parent *cobra.Command) []*cobra.Command {
	out := make([]*cobra.Command, 0)
	for _, cmd := range parent.Commands() {
		if !cmd.IsAvailableCommand() || cmd.Hidden {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func commandNames(commands []*cobra.Command) []string {
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Name())
	}
	return names
}

func commandContractNames(commands []commandContract) []string {
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.name)
	}
	return names
}

func mustFindCommand(t *testing.T, root *cobra.Command, path string) *cobra.Command {
	t.Helper()
	args := strings.Fields(path)
	cmd, remaining, err := root.Find(args)
	if err != nil {
		t.Fatalf("find %q: %v", path, err)
	}
	if len(remaining) != 0 {
		t.Fatalf("find %q left remaining args %v", path, remaining)
	}
	return cmd
}

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type discoverySummary struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type flagSummary struct {
	Name string `json:"name"`
}

type committedCLIContract struct {
	SchemaVersion int                        `json:"schema_version"`
	Commands      []committedCommandContract `json:"commands"`
}

type committedCommandContract struct {
	Path    string                  `json:"path"`
	Use     string                  `json:"use"`
	Aliases []string                `json:"aliases,omitempty"`
	Flags   []committedFlagContract `json:"flags,omitempty"`
}

type committedFlagContract struct {
	Name       string `json:"name"`
	Shorthand  string `json:"shorthand,omitempty"`
	Type       string `json:"type"`
	Default    string `json:"default"`
	Persistent bool   `json:"persistent,omitempty"`
}

func TestCommittedCLIContractMatchesLiveCommandTree(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")
	committed := loadCommittedCLIContract(t)
	live := collectLiveCLIContract(rootCmd)

	committedByPath := make(map[string]committedCommandContract, len(committed.Commands))
	for _, command := range committed.Commands {
		committedByPath[command.Path] = command
	}
	liveByPath := make(map[string]committedCommandContract, len(live))
	for _, command := range live {
		liveByPath[command.Path] = command
	}

	committedPaths := sortedKeys(committedByPath)
	livePaths := sortedKeys(liveByPath)
	if !slices.Equal(committedPaths, livePaths) {
		t.Fatalf("command paths differ:\ncommitted: %v\nlive: %v", committedPaths, livePaths)
	}

	for _, path := range committedPaths {
		assertCommandContractsEqual(t, path, committedByPath[path], liveByPath[path])
	}
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

func loadCommittedCLIContract(t *testing.T) committedCLIContract {
	t.Helper()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "contracts", "cli-v1.json"))
	if err != nil {
		t.Fatalf("read committed CLI contract: %v", err)
	}

	var contract committedCLIContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("decode committed CLI contract: %v", err)
	}
	if contract.SchemaVersion != 1 || len(contract.Commands) == 0 {
		t.Fatalf("incomplete committed CLI contract: %#v", contract)
	}
	return contract
}

func collectLiveCLIContract(root *cobra.Command) []committedCommandContract {
	var commands []committedCommandContract
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		cmd.InitDefaultHelpFlag()
		if cmd == root {
			cmd.InitDefaultVersionFlag()
		}

		flags := append(
			flagsFromSet(cmd.LocalNonPersistentFlags(), false),
			flagsFromSet(cmd.PersistentFlags(), true)...,
		)
		slices.SortFunc(
			flags,
			func(a, b committedFlagContract) int { return strings.Compare(a.Name, b.Name) },
		)
		commands = append(commands, committedCommandContract{
			Path:    cmd.CommandPath(),
			Use:     cmd.Use,
			Aliases: append([]string(nil), cmd.Aliases...),
			Flags:   flags,
		})

		children := make([]*cobra.Command, 0)
		for _, child := range cmd.Commands() {
			if child.Hidden || !child.IsAvailableCommand() {
				continue
			}
			children = append(children, child)
		}
		slices.SortFunc(
			children,
			func(a, b *cobra.Command) int { return strings.Compare(a.Name(), b.Name()) },
		)
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
	slices.SortFunc(
		commands,
		func(a, b committedCommandContract) int { return strings.Compare(a.Path, b.Path) },
	)
	return commands
}

func flagsFromSet(set *pflag.FlagSet, persistent bool) []committedFlagContract {
	if set == nil {
		return nil
	}
	var flags []committedFlagContract
	set.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, committedFlagContract{
			Name:       flag.Name,
			Shorthand:  flag.Shorthand,
			Type:       flag.Value.Type(),
			Default:    flag.DefValue,
			Persistent: persistent,
		})
	})
	return flags
}

func assertCommandContractsEqual(
	t *testing.T,
	path string,
	committed, live committedCommandContract,
) {
	t.Helper()

	if committed.Use != live.Use {
		t.Fatalf("%s use: committed %q, live %q", path, committed.Use, live.Use)
	}
	if !slices.Equal(committed.Aliases, live.Aliases) {
		t.Fatalf("%s aliases: committed %v, live %v", path, committed.Aliases, live.Aliases)
	}
	if len(committed.Flags) != len(live.Flags) {
		t.Fatalf("%s flags: committed %#v, live %#v", path, committed.Flags, live.Flags)
	}
	for i := range committed.Flags {
		if committed.Flags[i] != live.Flags[i] {
			t.Fatalf("%s flag %d: committed %#v, live %#v", path, i, committed.Flags[i], live.Flags[i])
		}
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
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

package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandPublicSurface(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	expected := []commandContract{
		{name: "build"},
		{name: "console", aliases: []string{"c"}},
		{name: "database", aliases: []string{"d", "db"}},
		{name: "doctor", aliases: []string{"doc"}},
		{name: "extension", aliases: []string{"ext", "e"}},
		{name: "fmt", aliases: []string{"f"}},
		{name: "generate", aliases: []string{"g"}},
		{name: "llm", aliases: []string{"l"}},
		{name: "new", aliases: []string{"n"}},
		{name: "run", aliases: []string{"r"}},
		{name: "tool", aliases: []string{"t"}},
		{name: "upgrade", aliases: []string{"up"}},
	}

	assertCommandSurface(t, rootCmd, expected)
}

func TestGenerateCommandPublicSurface(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")
	generateCmd := mustFindCommand(t, rootCmd, "generate")

	expected := []commandContract{
		{name: "controller", aliases: []string{"c"}},
		{name: "email", aliases: []string{"e"}},
		{name: "job", aliases: []string{"j"}},
		{name: "model", aliases: []string{"m"}},
		{name: "scaffold", aliases: []string{"s"}},
		{name: "view", aliases: []string{"v"}},
	}

	assertCommandSurface(t, generateCmd, expected)
}

func TestCommandFlagsContract(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		path  string
		flags []string
	}{
		{path: "new", flags: []string{"css", "extensions", "di", "inertia"}},
		{path: "generate model", flags: []string{"skip-factory", "table-name", "update", "yes", "primary-key"}},
		{path: "generate controller", flags: []string{"skip-routes", "vue"}},
		{path: "generate scaffold", flags: []string{"skip-factory", "table-name", "primary-key", "vue"}},
		{path: "generate job", flags: []string{"queue"}},
		{path: "fmt", flags: []string{"check", "skip-templ", "skip-go"}},
		{path: "database drop", flags: []string{"force"}},
		{path: "database nuke", flags: []string{"force"}},
		{path: "database rebuild", flags: []string{"force", "skip-seed"}},
		{path: "build", flags: []string{"version"}},
		{path: "doctor", flags: []string{"verbose"}},
		{path: "upgrade", flags: []string{"dry-run"}},
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

package cli

import (
	"bytes"
	"encoding/json"
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
		t.Fatalf(
			"commands --json returned error: %v\nstderr:\n%s",
			jsonResult.err,
			jsonResult.stderr,
		)
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
	if !discoveryContains(envelope.Data.Subcommands, "migration", "andurel generate migration") {
		t.Fatalf("expected migration subcommand in help: %#v", envelope.Data.Subcommands)
	}
}

func TestNewInertiaAndSQLCCommandsExposeAgentNotes(t *testing.T) {
	for _, path := range []string{
		"patterns",
		"verify",
		"generate migration",
		"sync queries",
		"inspect domains",
		"inspect handlers",
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

func TestGroupCommandsDispatchWithoutDefaultAction(t *testing.T) {
	// Root prints banner+help via Run. inspect project is a leaf with an
	// optional info alias. Every other command that owns subcommands uses the
	// group dispatch RunE: bare invocation shows help, an unknown subcommand is
	// a usage error. No group has a default action Run.
	allowRunnableWithChildren := map[string]bool{
		"andurel":                 true,
		"andurel inspect project": true,
	}

	root := NewRootCommand("test", "test-date")
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		t.Helper()
		children := availableSubcommands(cmd)
		if len(children) > 0 && !allowRunnableWithChildren[cmd.CommandPath()] {
			if cmd.Run != nil {
				t.Fatalf("%s has subcommands and a default Run action", cmd.CommandPath())
			}
			if cmd.RunE == nil {
				t.Fatalf("%s has subcommands but no group dispatch RunE", cmd.CommandPath())
			}
		}
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
}

func TestCommandsDiscoveryShortMatchesHelp(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	for _, cmd := range walkPublicCommands(root) {
		if cmd == root {
			continue
		}
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Help(); err != nil {
			t.Fatalf("%s help: %v", cmd.CommandPath(), err)
		}
		helpShort := strings.TrimSpace(strings.SplitN(out.String(), "\n", 2)[0])
		if got := discoverCommand(cmd).Short; got != helpShort {
			t.Errorf(
				"%s: commands short %q != help short %q",
				cmd.CommandPath(),
				got,
				helpShort,
			)
		}
	}
}

func TestCommandsSubcommandsExposeMeta(t *testing.T) {
	root := NewRootCommand("test", "test-date")
	discovery := discoverCommandTree(root)

	var update *commandSummary
	walk := func(commands []commandSummary) {
		for i := range commands {
			if commands[i].Path == "andurel packages update" {
				update = &commands[i]
			}
		}
	}
	walk(discovery.Subcommands)
	for _, child := range discovery.Commands {
		walk(child.Subcommands)
	}
	if update == nil {
		t.Fatal("missing andurel packages update summary")
	}
	if update.Meta == nil {
		t.Fatalf("andurel packages update has no meta: %#v", update)
	}
	if !update.Meta.Mutating || !update.Meta.SupportsDryRun || !update.Meta.SupportsJSON {
		t.Fatalf("andurel packages update meta = %#v", update.Meta)
	}

	var seed *commandSummary
	findAll := func(commands []commandSummary) {
		for i := range commands {
			if commands[i].Path == "andurel db seed" {
				seed = &commands[i]
			}
		}
	}
	findAll(discovery.Subcommands)
	for _, child := range discovery.Commands {
		findAll(child.Subcommands)
	}
	if seed == nil || seed.Meta == nil || !seed.Meta.SupportsJSON {
		t.Fatalf("andurel db seed meta = %#v", seed)
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

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateCommands(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		name string
		args []string
	}{
		{"generate model help", []string{"generate", "model", "--help"}},
		{"generate view help", []string{"generate", "view", "--help"}},
		{"generate controller help", []string{"generate", "controller", "--help"}},
		{"generate scaffold help", []string{"generate", "scaffold", "--help"}},
		{"generate job help", []string{"generate", "job", "--help"}},
		{"generate email help", []string{"generate", "email", "--help"}},
		{"fmt help", []string{"fmt", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Errorf("Command %v failed: %v", tt.args, err)
			}
		})
	}
}

func TestRootCommandStructure(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	expectedCommands := []string{"generate", "fmt"}
	foundCommands := make(map[string]bool)

	for _, cmd := range rootCmd.Commands() {
		cmdName := strings.Fields(cmd.Use)[0]
		foundCommands[cmdName] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !foundCommands[expectedCmd] {
			t.Errorf(
				"Expected root command '%s' not found. Available commands: %v",
				expectedCmd,
				getCommandNames(rootCmd.Commands()),
			)
		}
	}
}

func TestGenerateSubCommands(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	generateCmd, _, err := rootCmd.Find([]string{"generate"})
	if err != nil {
		t.Fatalf("'generate' command not found: %v", err)
	}

	expectedSubs := []string{"model", "view", "controller", "scaffold", "job", "email"}
	subNames := getCommandNames(generateCmd.Commands())

	for _, expectedSub := range expectedSubs {
		found := false
		for _, name := range subNames {
			if name == expectedSub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected generate subcommand '%s' not found. Available: %v", expectedSub, subNames)
		}
	}
}

func getCommandNames(commands []*cobra.Command) []string {
	var names []string
	for _, cmd := range commands {
		cmdName := strings.Fields(cmd.Use)[0]
		names = append(names, cmdName)
	}
	return names
}

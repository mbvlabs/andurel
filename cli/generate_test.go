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
		{"model help", []string{"model", "--help"}},
		{"controller help", []string{"controller", "--help"}},
		{"view help", []string{"view", "--help"}},
		{"resource help", []string{"resource", "--help"}},
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

	expectedCommands := []string{"controller", "view", "resource", "model"}
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

func getCommandNames(commands []*cobra.Command) []string {
	var names []string
	for _, cmd := range commands {
		cmdName := strings.Fields(cmd.Use)[0]
		names = append(names, cmdName)
	}
	return names
}

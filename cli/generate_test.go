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
		{"generate help", []string{"generate", "--help"}},
		{"model help", []string{"generate", "model", "--help"}},
		{"controller help", []string{"generate", "controller", "--help"}},
		{"resource help", []string{"generate", "resource", "--help"}},
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

func TestGenerateCommandStructure(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	// Test generate command structure
	var generateCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "generate" {
			generateCmd = cmd
			break
		}
	}

	if generateCmd == nil {
		t.Fatal("generate command not found")
	}

	expectedCommands := []string{"model", "controller", "view", "resource"}
	foundCommands := make(map[string]bool)

	for _, cmd := range generateCmd.Commands() {
		cmdName := strings.Fields(cmd.Use)[0]
		foundCommands[cmdName] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !foundCommands[expectedCmd] {
			t.Errorf(
				"Expected command '%s' not found. Available commands: %v",
				expectedCmd,
				getCommandNames(generateCmd.Commands()),
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

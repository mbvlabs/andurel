package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestViewCommands(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		name string
		args []string
	}{
		{"view help", []string{"view", "--help"}},
		{"view generate help", []string{"view", "generate", "--help"}},
		{"view compile alias help", []string{"view", "compile", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			if err := rootCmd.Execute(); err != nil {
				t.Errorf("command %v failed: %v", tt.args, err)
			}
		})
	}
}

func TestViewCommandStructure(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	var viewCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "view" {
			viewCmd = cmd
			break
		}
	}

	if viewCmd == nil {
		t.Fatal("view command not found")
	}

	expectedCommands := []string{"generate", "format"}
	foundCommands := make(map[string]bool)

	for _, cmd := range viewCmd.Commands() {
		foundCommands[cmd.Name()] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !foundCommands[expectedCmd] {
			t.Errorf(
				"expected command %q not found. Available commands: %v",
				expectedCmd,
				getCommandNames(viewCmd.Commands()),
			)
		}
	}

	generateCmd, _, err := viewCmd.Find([]string{"generate"})
	if err != nil {
		t.Fatalf("failed to find generate command: %v", err)
	}
	if !generateCmd.HasAlias("compile") {
		t.Fatal("generate command should keep compile as an alias")
	}
}

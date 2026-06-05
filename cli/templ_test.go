package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestFmtCommand(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		name string
		args []string
	}{
		{"fmt help", []string{"fmt", "--help"}},
		{"fmt check help", []string{"fmt", "--check", "--help"}},
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

func TestFmtCommandStructure(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	var fmtCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "fmt" {
			fmtCmd = cmd
			break
		}
	}

	if fmtCmd == nil {
		t.Fatal("fmt command not found")
	}

	if !fmtCmd.HasFlags() {
		t.Error("fmt command should have flags (--check, --skip-templ, --skip-go)")
	}

	checkFlag := fmtCmd.Flag("check")
	if checkFlag == nil {
		t.Error("fmt command should have --check flag")
	}

	skipTemplFlag := fmtCmd.Flag("skip-templ")
	if skipTemplFlag == nil {
		t.Error("fmt command should have --skip-templ flag")
	}

	skipGoFlag := fmtCmd.Flag("skip-go")
	if skipGoFlag == nil {
		t.Error("fmt command should have --skip-go flag")
	}
}

func TestGenerateViewsCommand(t *testing.T) {
	rootCmd := NewRootCommand("test", "test-date")

	tests := []struct {
		name string
		args []string
	}{
		{"generate view help", []string{"generate", "view", "--help"}},
		{"generate view alias v", []string{"generate", "v", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			if err := rootCmd.Execute(); err != nil {
				t.Errorf("command %v failed: %v", tt.args, err)
			}
		})
	}

	generateCmd, _, err := rootCmd.Find([]string{"generate"})
	if err != nil {
		t.Fatalf("'generate' command not found: %v", err)
	}

	viewCmd, _, err := generateCmd.Find([]string{"view"})
	if err != nil {
		t.Fatalf("'generate view' command not found: %v", err)
	}
	if !viewCmd.HasAlias("v") {
		t.Fatal("generate view command should have v as an alias")
	}
}

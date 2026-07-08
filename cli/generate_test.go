package cli

import (
	"slices"
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
		{"generate routes help", []string{"generate", "routes", "--help"}},
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

	expectedSubs := []string{"model", "view", "controller", "scaffold", "job", "email", "routes"}
	subNames := getCommandNames(generateCmd.Commands())

	for _, expectedSub := range expectedSubs {
		found := slices.Contains(subNames, expectedSub)
		if !found {
			t.Errorf("Expected generate subcommand '%s' not found. Available: %v", expectedSub, subNames)
		}
	}
}

func TestGenerateHelpMentionsNamespacedResources(t *testing.T) {
	generateCmd := newGenerateCommand()
	controllerCmd := newGenerateControllerCommand()
	scaffoldCmd := newGenerateScaffoldCommand()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "generate",
			text: generateCmd.Long + "\n" + generateCmd.Example,
			want: []string{"admin/Widget", "generate controller admin/Widget export", "generate scaffold admin/Widget", "generate routes"},
		},
		{
			name: "controller",
			text: controllerCmd.Long + "\n" + controllerCmd.Example,
			want: []string{"admin/Widget", "controllers/admin/widgets.go", "admin.widgets.export"},
		},
		{
			name: "scaffold",
			text: scaffoldCmd.Long + "\n" + scaffoldCmd.Example,
			want: []string{"admin/Widget", "controllers/admin", "views/admin_widgets_resource.templ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.want {
				if !strings.Contains(tt.text, want) {
					t.Fatalf("expected %s help to mention %q:\n%s", tt.name, want, tt.text)
				}
			}
		})
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

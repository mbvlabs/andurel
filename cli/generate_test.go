package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
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

func TestProjectScaffoldingSqlite__GoldenFile(t *testing.T) {
	tests := []struct {
		name           string
		projectName    string
		repoFlag       string
		expectedModule string
	}{
		{
			name:           "Should_scaffold_project_with_simple_name_sqlite",
			projectName:    "testapp",
			repoFlag:       "",
			expectedModule: "testapp",
		},
		{
			name:           "Should_scaffold_project_with_github_repo_sqlite",
			projectName:    "myapp",
			repoFlag:       "github.com/testuser",
			expectedModule: "github.com/testuser/myapp",
		},
		{
			name:           "Should_scaffold_project_with_simple_repo_sqlite",
			projectName:    "webapp",
			repoFlag:       "myorg",
			expectedModule: "myorg/webapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			projectDir := filepath.Join(tempDir, tt.projectName)

			originalWd, _ := os.Getwd()

			rootCmd := NewRootCommand("test", "test-date")

			args := []string{"new", tt.projectName, "--database", "sqlite"}
			if tt.repoFlag != "" {
				args = append(args, "--repo", tt.repoFlag)
			}

			rootCmd.SetArgs(args)

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("Project scaffolding failed: %v", err)
			}

			scaffoldOutput := captureScaffoldedProject(t, projectDir)

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".txt"))

			g.Assert(t, tt.name, []byte(scaffoldOutput))
		})
	}
}

func TestProjectScaffoldingPostgresql__GoldenFile(t *testing.T) {
	tests := []struct {
		name           string
		projectName    string
		repoFlag       string
		expectedModule string
	}{
		{
			name:           "Should_scaffold_project_with_simple_name",
			projectName:    "testapp",
			repoFlag:       "",
			expectedModule: "testapp",
		},
		{
			name:           "Should_scaffold_project_with_github_repo",
			projectName:    "myapp",
			repoFlag:       "github.com/testuser",
			expectedModule: "github.com/testuser/myapp",
		},
		{
			name:           "Should_scaffold_project_with_simple_repo",
			projectName:    "webapp",
			repoFlag:       "myorg",
			expectedModule: "myorg/webapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			projectDir := filepath.Join(tempDir, tt.projectName)

			originalWd, _ := os.Getwd()

			rootCmd := NewRootCommand("test", "test-date")

			args := []string{"new", tt.projectName}
			if tt.repoFlag != "" {
				args = append(args, "--repo", tt.repoFlag)
			}

			rootCmd.SetArgs(args)

			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tempDir)

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("Project scaffolding failed: %v", err)
			}

			scaffoldOutput := captureScaffoldedProject(t, projectDir)

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".txt"))

			g.Assert(t, tt.name, []byte(scaffoldOutput))
		})
	}
}

func captureScaffoldedProject(t *testing.T, projectDir string) string {
	var output strings.Builder
	var allFiles []string

	err := filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}

			if strings.Contains(relPath, ".git/") ||
				strings.HasSuffix(relPath, ".mod.sum") ||
				strings.HasSuffix(relPath, "go.sum") {
				return nil
			}

			allFiles = append(allFiles, relPath)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk project directory: %v", err)
	}

	sort.Strings(allFiles)

	output.WriteString("=== PROJECT STRUCTURE ===\n")
	for _, file := range allFiles {
		output.WriteString(file + "\n")
	}
	output.WriteString("\n")

	for _, file := range allFiles {
		filePath := filepath.Join(projectDir, file)

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file %s: %v", file, err)
		}

		contentStr := string(content)
		if strings.HasSuffix(file, ".env.example") {
			contentStr = normalizeEnvSecrets(contentStr)
		}

		output.WriteString(fmt.Sprintf("=== %s ===\n", file))
		output.WriteString(contentStr)
		output.WriteString("\n\n")
	}

	return output.String()
}

func normalizeEnvSecrets(content string) string {
	content = replaceEnvValue(content, "PASSWORD_SALT=", "test_password_salt_value")
	content = replaceEnvValue(content, "SESSION_KEY=", "test_session_key_value")
	content = replaceEnvValue(
		content,
		"SESSION_ENCRYPTION_KEY=",
		"test_session_encryption_key_value",
	)
	content = replaceEnvValue(content, "TOKEN_SIGNING_KEY=", "test_token_signing_key_value")
	return content
}

func replaceEnvValue(content, prefix, testValue string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + testValue
		}
	}
	return strings.Join(lines, "\n")
}

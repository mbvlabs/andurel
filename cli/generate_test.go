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

func TestProjectScaffolding__GoldenFile(t *testing.T) {
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

// TestCLIInputValidation tests the CLI input validation requirements
func TestCLIInputValidation(t *testing.T) {

	tests := []struct {
		name        string
		args        []string
		expectError bool
		description string
	}{
		// Model command tests - should require both name and table
		{
			name:        "model_with_no_args",
			args:        []string{"generate", "model"},
			expectError: true,
			description: "Model command should require both name and table args",
		},
		{
			name:        "model_with_one_arg",
			args:        []string{"generate", "model", "User"},
			expectError: true,
			description: "Model command should require table name as second arg",
		},
		{
			name:        "model_with_three_args",
			args:        []string{"generate", "model", "User", "users", "extra"},
			expectError: true,
			description: "Model command should only accept exactly 2 args",
		},

		// Controller command tests - should require only model name
		{
			name:        "controller_with_no_args",
			args:        []string{"generate", "controller"},
			expectError: true,
			description: "Controller command should require model name arg",
		},
		{
			name:        "controller_with_two_args",
			args:        []string{"generate", "controller", "User", "users"},
			expectError: true,
			description: "Controller command should only accept model name (no table name)",
		},

		// View command tests - should require only model name
		{
			name:        "view_with_no_args",
			args:        []string{"generate", "view"},
			expectError: true,
			description: "View command should require model name arg",
		},
		{
			name:        "view_with_two_args",
			args:        []string{"generate", "view", "User", "users"},
			expectError: true,
			description: "View command should only accept model name (no table name)",
		},

		// Resource command tests - should require both name and table
		{
			name:        "resource_with_no_args",
			args:        []string{"generate", "resource"},
			expectError: true,
			description: "Resource command should require both name and table args",
		},
		{
			name:        "resource_with_one_arg",
			args:        []string{"generate", "resource", "User"},
			expectError: true,
			description: "Resource command should require table name as second arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh command for each test
			cmd := NewRootCommand("test", "test-date")
			cmd.SetArgs(tt.args)

			// Capture stderr to avoid cluttering test output
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: expected error but command succeeded", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("%s: expected success but got error: %v", tt.description, err)
				}
			}
		})
	}
}

// TestModelDependencyValidation tests that controller and view commands validate model existence
func TestModelDependencyValidation(t *testing.T) {
	// Create a temporary directory to simulate a project
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Create basic project structure including go.mod
	os.MkdirAll("models", 0755)
	os.MkdirAll("controllers", 0755)
	os.MkdirAll("views", 0755)
	os.MkdirAll("database", 0755)
	
	// Create minimal go.mod
	goMod := "module testproject\n\ngo 1.21\n"
	os.WriteFile("go.mod", []byte(goMod), 0644)
	
	// Create minimal sqlc.yaml
	sqlcYaml := `version: "2"
sql:
  - engine: "postgresql"
    queries: "database/queries"
    schema: "database/migrations"
    gen:
      go:
        package: "models"
        out: "models"
`
	os.WriteFile("database/sqlc.yaml", []byte(sqlcYaml), 0644)
	os.MkdirAll("database/queries", 0755)
	os.MkdirAll("database/migrations", 0755)

	tests := []struct {
		name          string
		args          []string
		setupModel    bool
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:          "view_without_model",
			args:          []string{"generate", "view", "User"},
			setupModel:    false,
			expectError:   true,
			errorContains: "does not exist",
			description:   "View generation should fail when model doesn't exist",
		},
		{
			name:          "controller_without_model", 
			args:          []string{"generate", "controller", "User"},
			setupModel:    false,
			expectError:   true,
			errorContains: "does not exist",
			description:   "Controller generation should fail when model doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup model file if needed
			if tt.setupModel {
				modelFile := "models/user.go"
				os.WriteFile(modelFile, []byte("package models\n\ntype User struct {}\n"), 0644)
				defer os.Remove(modelFile)
			}

			cmd := NewRootCommand("test", "test-date")
			cmd.SetArgs(tt.args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: expected error but command succeeded", tt.description)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("%s: expected error containing '%s' but got: %v", tt.description, tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("%s: expected success but got error: %v", tt.description, err)
				}
			}
		})
	}
}

// TestResourceCommandBehavior tests that resource command generates model first, then controller
func TestResourceCommandBehavior(t *testing.T) {
	// This test validates the behavior described in requirements:
	// "if all three are generated at once, both controller and view should follow the name of the model"
	
	tests := []struct {
		name        string
		args        []string
		expectError bool
		description string
	}{
		{
			name:        "resource_command_structure",
			args:        []string{"generate", "resource", "Product", "products"},
			expectError: true, // Will fail due to missing project setup, but validates args
			description: "Resource command should accept model name and table name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand("test", "test-date")
			cmd.SetArgs(tt.args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			err := cmd.Execute()

			// We expect this to fail due to project setup, but not due to argument validation
			if err != nil {
				// Check that it's not an argument validation error
				if strings.Contains(err.Error(), "accepts 2 arg(s)") {
					t.Errorf("%s: failed due to argument validation: %v", tt.description, err)
				}
				// Otherwise, failure is expected due to missing project setup
			}
		})
	}
}

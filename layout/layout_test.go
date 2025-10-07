package layout_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout"
	"github.com/sebdah/goldie/v2"
)

func TestProjectScaffoldingSqlite__GoldenFile(t *testing.T) {
	if err := os.Setenv("ANDUREL_SKIP_TAILWIND", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_TAILWIND env var: %v", err)
	}
	if err := os.Setenv("ANDUREL_SKIP_BUILD", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_TAILWIND env var: %v", err)
	}

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
			expectedModule: "testuser/myapp",
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

			err := layout.Scaffold(projectDir, tt.projectName, tt.repoFlag, "sqlite", nil)
			if err != nil {
				t.Fatalf("Failed to scaffold project: %v", err)
			}

			scaffoldOutput := captureScaffoldedProject(t, projectDir)

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".txt"))

			g.Assert(t, tt.name, []byte(scaffoldOutput))
		})
	}
}

func TestProjectScaffoldingPostgresql__GoldenFile(t *testing.T) {
	if err := os.Setenv("ANDUREL_SKIP_TAILWIND", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_TAILWIND env var: %v", err)
	}
	if err := os.Setenv("ANDUREL_SKIP_BUILD", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_TAILWIND env var: %v", err)
	}
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
			expectedModule: "testuser/myapp",
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

			err := layout.Scaffold(projectDir, tt.projectName, tt.repoFlag, "postgresql", nil)
			if err != nil {
				t.Fatalf("Failed to scaffold project: %v", err)
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
		if strings.HasSuffix(file, ".env.example") || strings.HasSuffix(file, ".env") {
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

func TestProjectScaffoldingWithExtensions__GoldenFile(t *testing.T) {
	if err := os.Setenv("ANDUREL_SKIP_TAILWIND", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_TAILWIND env var: %v", err)
	}
	if err := os.Setenv("ANDUREL_SKIP_BUILD", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_BUILD env var: %v", err)
	}

	tests := []struct {
		name           string
		projectName    string
		repoFlag       string
		database       string
		extensions     []string
		expectedModule string
	}{
		{
			name:           "Should_scaffold_project_with_simple_auth_postgresql",
			projectName:    "testapp",
			repoFlag:       "",
			database:       "postgresql",
			extensions:     []string{"simple-auth"},
			expectedModule: "testapp",
		},
		{
			name:           "Should_scaffold_project_with_simple_auth_sqlite",
			projectName:    "testapp",
			repoFlag:       "",
			database:       "sqlite",
			extensions:     []string{"simple-auth"},
			expectedModule: "testapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			projectDir := filepath.Join(tempDir, tt.projectName)

			originalWd, _ := os.Getwd()

			err := layout.Scaffold(projectDir, tt.projectName, tt.repoFlag, tt.database, tt.extensions)
			if err != nil {
				t.Fatalf("Failed to scaffold project: %v", err)
			}

			scaffoldOutput := captureScaffoldedProject(t, projectDir)

			fixtureDir := filepath.Join(originalWd, "testdata")
			g := goldie.New(t, goldie.WithFixtureDir(fixtureDir), goldie.WithNameSuffix(".txt"))

			g.Assert(t, tt.name, []byte(scaffoldOutput))
		})
	}
}

func TestScaffold_WithEmailExtension(t *testing.T) {
	if err := os.Setenv("ANDUREL_SKIP_TAILWIND", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_TAILWIND env var: %v", err)
	}
	if err := os.Setenv("ANDUREL_SKIP_BUILD", "true"); err != nil {
		t.Fatalf("Failed to set ANDUREL_SKIP_BUILD env var: %v", err)
	}

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "test-email-app")

	err := layout.Scaffold(targetDir, "test-email-app", "", "sqlite", []string{"email"})
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	// Check email package was created
	emailFile := filepath.Join(targetDir, "email", "email.go")
	if _, err := os.Stat(emailFile); os.IsNotExist(err) {
		t.Error("email/email.go was not created")
	}

	mailHogFile := filepath.Join(targetDir, "email", "mail_hog.go")
	if _, err := os.Stat(mailHogFile); os.IsNotExist(err) {
		t.Error("email/mail_hog.go was not created")
	}

	// Check main.go contains email initialization
	mainFile := filepath.Join(targetDir, "cmd", "app", "main.go")
	content, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("failed to read main.go: %v", err)
	}

	mainContent := string(content)

	// Check for email import
	if !strings.Contains(mainContent, "test-email-app/email") {
		t.Error("main.go missing email import")
	}

	// Check for email initialization
	if !strings.Contains(mainContent, "emailSender := email.NewMailHog()") {
		t.Error("main.go missing emailSender initialization")
	}

	// Check for emailSender in setupControllers call
	if !strings.Contains(mainContent, "emailSender,") {
		t.Error("main.go missing emailSender in setupControllers")
	}
}

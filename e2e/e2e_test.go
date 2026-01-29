package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout/versions"
)

var (
	andurelBinary string
	sharedBinDir  string
)

// requiredTools lists the tools needed for e2e tests with their go install paths
var requiredTools = []struct {
	name    string
	module  string
	version string
}{
	{"sqlc", "github.com/sqlc-dev/sqlc/cmd/sqlc", versions.Sqlc},
	{"templ", "github.com/a-h/templ/cmd/templ", versions.Templ},
	{"goose", "github.com/pressly/goose/v3/cmd/goose", versions.Goose},
}

func TestMain(m *testing.M) {
	workDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get working directory: %v", err))
	}

	projectRoot := filepath.Dir(workDir)

	tmpDir, err := os.MkdirTemp("", "andurel-e2e-*")
	if err != nil {
		panic(fmt.Sprintf("Failed to create temp directory: %v", err))
	}
	defer os.RemoveAll(tmpDir)

	andurelBinary = filepath.Join(tmpDir, "andurel")

	cmd := exec.Command("go", "build", "-o", andurelBinary, ".")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("Failed to build andurel binary: %v\n%s", err, output))
	}

	// Set up shared bin directory for tools
	sharedBinDir = filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(sharedBinDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create shared bin directory: %v", err))
	}

	// Download required tools once
	for _, tool := range requiredTools {
		binPath := filepath.Join(sharedBinDir, tool.name)
		fmt.Printf("Downloading %s@%s...\n", tool.name, tool.version)

		cmd := exec.Command("go", "build", "-o", binPath, tool.module+"@"+tool.version)
		cmd.Env = append(os.Environ(), "GOBIN="+sharedBinDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("Failed to download %s: %v\n%s", tool.name, err, output))
		}
	}

	code := m.Run()
	os.Exit(code)
}

func buildAndurelBinary(t *testing.T) string {
	t.Helper()
	return andurelBinary
}

func getSharedBinDir() string {
	return sharedBinDir
}

func isCriticalOnly() bool {
	return os.Getenv("E2E_CRITICAL_ONLY") == "true"
}

// TestToolVersions validates that the tool versions in layout/versions match what
// the tools actually report. This ensures our version constants stay in sync.
func TestToolVersions(t *testing.T) {
	if sharedBinDir == "" {
		t.Skip("Shared bin directory not available")
	}

	tests := []struct {
		name            string
		binary          string
		versionFlag     string
		expectedVersion string
		versionParser   func(output string) string
	}{
		{
			name:            "sqlc",
			binary:          "sqlc",
			versionFlag:     "version",
			expectedVersion: versions.Sqlc,
			versionParser: func(output string) string {
				// sqlc outputs: "v1.30.0"
				return strings.TrimSpace(output)
			},
		},
		{
			name:            "templ",
			binary:          "templ",
			versionFlag:     "version",
			expectedVersion: versions.Templ,
			versionParser: func(output string) string {
				// templ outputs: "templ version v0.3.960"
				parts := strings.Fields(output)
				if len(parts) >= 3 {
					return parts[2]
				}
				return strings.TrimSpace(output)
			},
		},
		{
			name:            "goose",
			binary:          "goose",
			versionFlag:     "--version",
			expectedVersion: versions.Goose,
			versionParser: func(output string) string {
				// goose outputs: "goose version: v3.24.1"
				parts := strings.Fields(output)
				if len(parts) >= 3 {
					return parts[2]
				}
				return strings.TrimSpace(output)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binPath := filepath.Join(sharedBinDir, tc.binary)
			cmd := exec.Command(binPath, tc.versionFlag)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to get %s version: %v\nOutput: %s", tc.name, err, output)
			}

			actualVersion := tc.versionParser(string(output))
			if actualVersion != tc.expectedVersion {
				t.Errorf("%s version mismatch: expected %s, got %s",
					tc.name, tc.expectedVersion, actualVersion)
			}
		})
	}
}

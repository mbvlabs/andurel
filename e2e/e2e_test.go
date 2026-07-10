package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// Download required tools once using go install
	for _, tool := range requiredTools {
		fmt.Printf("Downloading %s@%s...\n", tool.name, tool.version)

		cmd := exec.Command("go", "install", tool.module+"@"+tool.version)
		cmd.Env = append(os.Environ(), "GOBIN="+sharedBinDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("Failed to download %s: %v\n%s", tool.name, err, output))
		}
	}

	code := m.Run()
	if err := os.RemoveAll(tmpDir); err != nil {
		if _, writeErr := fmt.Fprintf(os.Stderr, "Failed to clean E2E temporary directory: %v\n", err); writeErr != nil {
			code = 1
		}
		code = 1
	}
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

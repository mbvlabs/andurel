package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var andurelBinary string

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

	code := m.Run()
	os.Exit(code)
}

func buildAndurelBinary(t *testing.T) string {
	t.Helper()
	return andurelBinary
}

func isCriticalOnly() bool {
	return os.Getenv("E2E_CRITICAL_ONLY") == "true"
}

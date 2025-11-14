package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
)

var andurelBinary string

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func buildAndurelBinary(t *testing.T) string {
	t.Helper()

	if andurelBinary != "" {
		return andurelBinary
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Dir(workDir)

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "andurel")

	err = internal.RunCommand(t, "go", projectRoot, nil, "build", "-o", binaryPath, ".")
	if err != nil {
		t.Fatalf("Failed to build andurel binary: %v", err)
	}

	andurelBinary = binaryPath
	return andurelBinary
}

func isCriticalOnly() bool {
	return os.Getenv("E2E_CRITICAL_ONLY") == "true"
}

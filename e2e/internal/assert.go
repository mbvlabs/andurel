package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AssertFileExists performs assert file exists.
func AssertFileExists(t *testing.T, p *Project, path string) {
	t.Helper()

	if !p.FileExists(path) {
		t.Errorf("Expected file to exist: %s", path)
	}
}

// AssertDirExists performs assert dir exists.
func AssertDirExists(t *testing.T, p *Project, path string) {
	t.Helper()

	if !p.DirExists(path) {
		t.Errorf("Expected directory to exist: %s", path)
	}
}

// AssertFilesExist performs assert files exist.
func AssertFilesExist(t *testing.T, p *Project, paths []string) {
	t.Helper()

	for _, path := range paths {
		AssertFileExists(t, p, path)
	}
}

// AssertGoVetPasses performs assert go vet passes.
func AssertGoVetPasses(t *testing.T, p *Project) {
	t.Helper()

	if err := p.GoVet(); err != nil {
		t.Fatalf("go vet failed: %v", err)
	}
}

// AssertCommandSucceeds performs assert command succeeds.
func AssertCommandSucceeds(t *testing.T, err error, cmdDesc string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s failed: %v", cmdDesc, err)
	}
}

// AssertOutputContains performs assert output contains.
func AssertOutputContains(t *testing.T, output, expected string) {
	t.Helper()

	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
	}
}

// AssertFileContains performs assert file contains.
func AssertFileContains(t *testing.T, p *Project, path, expected string) {
	t.Helper()

	fullPath := filepath.Join(p.Dir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	if !strings.Contains(string(content), expected) {
		t.Errorf("Expected file %s to contain %q", path, expected)
	}
}

package internal

import (
	"strings"
	"testing"
)

func AssertFileExists(t *testing.T, p *Project, path string) {
	t.Helper()

	if !p.FileExists(path) {
		t.Errorf("Expected file to exist: %s", path)
	}
}

func AssertDirExists(t *testing.T, p *Project, path string) {
	t.Helper()

	if !p.DirExists(path) {
		t.Errorf("Expected directory to exist: %s", path)
	}
}

func AssertFilesExist(t *testing.T, p *Project, paths []string) {
	t.Helper()

	for _, path := range paths {
		AssertFileExists(t, p, path)
	}
}

func AssertGoVetPasses(t *testing.T, p *Project) {
	t.Helper()

	if err := p.GoVet(); err != nil {
		t.Fatalf("go vet failed: %v", err)
	}
}

func AssertCommandSucceeds(t *testing.T, err error, cmdDesc string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s failed: %v", cmdDesc, err)
	}
}

func AssertOutputContains(t *testing.T, output, expected string) {
	t.Helper()

	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
	}
}

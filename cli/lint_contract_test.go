package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintCommandPinsVersionAndDeterministicConfiguration(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "lint.sh"))
	if err != nil {
		t.Fatalf("read lint command: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(root, ".golangci.yml"))
	if err != nil {
		t.Fatalf("read lint configuration: %v", err)
	}

	for _, required := range []string{
		`expected_version="2.12.2"`,
		"GOLANGCI_LINT_CACHE=",
		"GOCACHE=",
		"golangci-lint run --config .golangci.yml",
	} {
		if !strings.Contains(string(script), required) {
			t.Fatalf("lint command missing %q", required)
		}
	}
	for _, required := range []string{`version: "2"`, "modules-download-mode: readonly", "errcheck", "staticcheck", "unused"} {
		if !strings.Contains(string(config), required) {
			t.Fatalf("lint configuration missing %q", required)
		}
	}
}

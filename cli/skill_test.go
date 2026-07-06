package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/skills"
)

func TestSkillInstallWritesProjectLocalSkill(t *testing.T) {
	resetCLITestSeams(t)

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	findGoModRoot = func() (string, error) {
		return rootDir, nil
	}

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"skill", "install", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install failed: %v\nstderr:\n%s", err, stderr.String())
	}

	var envelope struct {
		OK   bool        `json:"ok"`
		Data skillReport `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode skill install output: %v\nstdout:\n%s", err, stdout.String())
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope: %#v", envelope)
	}

	expectedPath := filepath.Join(rootDir, ".codex", "skills", "andurel", "SKILL.md")
	if envelope.Data.Path != expectedPath {
		t.Fatalf("expected project-local skill path %q, got %q", expectedPath, envelope.Data.Path)
	}

	data, err := os.ReadFile(envelope.Data.Path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(data) != skills.AndurelSkill {
		t.Fatalf("installed skill content does not match embedded skill")
	}
}

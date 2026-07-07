package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/skills"
)

func TestSkillShowPrintsEmbeddedSkillInHumanMode(t *testing.T) {
	result := runCLITest(t, "skill", "show")
	if result.err != nil {
		t.Fatalf("skill show failed: %v\nstderr:\n%s", result.err, result.stderr)
	}
	if result.stdout != skills.AndurelSkill {
		t.Fatalf("expected raw skill body, got:\n%s", result.stdout)
	}
	if strings.Contains(result.stdout, "Loaded Andurel skill") {
		t.Fatalf("human skill show should not print summary text:\n%s", result.stdout)
	}
}

func TestSkillShowJSONIncludesEmbeddedSkill(t *testing.T) {
	result := runCLITest(t, "skill", "show", "--json")
	if result.err != nil {
		t.Fatalf("skill show --json failed: %v\nstderr:\n%s", result.err, result.stderr)
	}

	var envelope struct {
		OK   bool        `json:"ok"`
		Data skillReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
		t.Fatalf("decode skill show output: %v\nstdout:\n%s", err, result.stdout)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope: %#v", envelope)
	}
	if envelope.Data.Name != "andurel" || envelope.Data.Body != skills.AndurelSkill {
		t.Fatalf("unexpected skill show data: %#v", envelope.Data)
	}
}

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

	referencePath := filepath.Join(rootDir, ".codex", "skills", "andurel", "references", "layer-placement.md")
	referenceData, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("read installed layer placement reference: %v", err)
	}
	if !strings.Contains(string(referenceData), "# Layer Placement") {
		t.Fatalf("installed layer placement reference has unexpected content:\n%s", referenceData)
	}

	agentPath := filepath.Join(rootDir, ".codex", "skills", "andurel", "agents", "openai.yaml")
	agentData, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("read installed OpenAI agent metadata: %v", err)
	}
	if !strings.Contains(string(agentData), `display_name: "Andurel"`) {
		t.Fatalf("installed OpenAI agent metadata has unexpected content:\n%s", agentData)
	}
}

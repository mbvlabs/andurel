package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
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

func TestSkillInstallPromptsForHarnesses(t *testing.T) {
	result := runSkillCommandTest(t, "2,3,5\n", "skill", "install")
	if result.err != nil {
		t.Fatalf("skill install failed: %v\nstderr:\n%s", result.err, result.stderr)
	}
	for _, option := range []string{"1) Codex", "2) Claude", "3) Pi", "4) OpenCode", "5) Crush", "Selection:"} {
		if !strings.Contains(result.stdout, option) {
			t.Fatalf("interactive output missing %q:\n%s", option, result.stdout)
		}
	}

	for _, harness := range []string{"claude", "pi", "crush"} {
		assertInstalledSkill(t, result.rootDir, harnessSkillDirectories[harness])
	}
	for _, harness := range []string{"codex", "opencode"} {
		assertSkillNotInstalled(t, result.rootDir, harnessSkillDirectories[harness])
	}
}

func TestSkillInstallHarnessFlagWritesEachHarness(t *testing.T) {
	for harness, directory := range harnessSkillDirectories {
		t.Run(harness, func(t *testing.T) {
			result := runSkillCommandTest(t, "", "skill", "install", "--harness", harness, "--json")
			if result.err != nil {
				t.Fatalf("skill install failed: %v\nstderr:\n%s", result.err, result.stderr)
			}

			var envelope struct {
				OK   bool `json:"ok"`
				Data struct {
					Path          string `json:"path"`
					Installations []struct {
						Harness string `json:"harness"`
						Path    string `json:"path"`
					} `json:"installations"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
				t.Fatalf("decode skill install output: %v\nstdout:\n%s", err, result.stdout)
			}
			expectedPath := filepath.Join(result.rootDir, filepath.FromSlash(directory), "SKILL.md")
			if !envelope.OK || envelope.Data.Path != expectedPath {
				t.Fatalf("unexpected install envelope: %#v", envelope)
			}
			if len(envelope.Data.Installations) != 1 ||
				envelope.Data.Installations[0].Harness != harness ||
				envelope.Data.Installations[0].Path != expectedPath {
				t.Fatalf("unexpected installations: %#v", envelope.Data.Installations)
			}
			assertInstalledSkill(t, result.rootDir, directory)
		})
	}
}

func TestSkillInstallHarnessFlagSupportsMultipleSelections(t *testing.T) {
	result := runSkillCommandTest(
		t,
		"",
		"skill", "install",
		"--harness", "Claude,pi",
		"--harness", "crush,claude",
		"--json",
	)
	if result.err != nil {
		t.Fatalf("skill install failed: %v\nstderr:\n%s", result.err, result.stderr)
	}

	var envelope struct {
		Data struct {
			Path          string `json:"path"`
			Installations []struct {
				Harness string `json:"harness"`
				Path    string `json:"path"`
			} `json:"installations"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
		t.Fatalf("decode skill install output: %v\nstdout:\n%s", err, result.stdout)
	}
	if envelope.Data.Path != "" {
		t.Fatalf("multi-harness install should omit singular path, got %q", envelope.Data.Path)
	}
	want := []string{"claude", "pi", "crush"}
	if len(envelope.Data.Installations) != len(want) {
		t.Fatalf("installations = %#v, want %v", envelope.Data.Installations, want)
	}
	for index, harness := range want {
		if envelope.Data.Installations[index].Harness != harness {
			t.Fatalf("installation %d = %#v, want harness %q", index, envelope.Data.Installations[index], harness)
		}
		assertInstalledSkill(t, result.rootDir, harnessSkillDirectories[harness])
	}
}

func TestSkillInstallRejectsInvalidSelectionsBeforeWriting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		args  []string
	}{
		{name: "blank prompt", input: "\n", args: []string{"skill", "install"}},
		{name: "out of range prompt", input: "6\n", args: []string{"skill", "install"}},
		{name: "unknown flag", args: []string{"skill", "install", "--harness", "other", "--json"}},
		{name: "structured without flag", args: []string{"skill", "install", "--json"}},
		{name: "quiet without flag", args: []string{"skill", "install", "--quiet"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runSkillCommandTest(t, test.input, test.args...)
			if result.err == nil {
				t.Fatalf("expected skill install to fail\nstdout:\n%s", result.stdout)
			}
			if fail := output.Fail(result.err); fail.Code != output.CodeUsage {
				t.Fatalf("error code = %q, want %q: %v", fail.Code, output.CodeUsage, result.err)
			}
			for _, directory := range harnessSkillDirectories {
				assertSkillNotInstalled(t, result.rootDir, directory)
			}
		})
	}
}

type skillCommandTestResult struct {
	rootDir string
	stdout  string
	stderr  string
	err     error
}

var harnessSkillDirectories = map[string]string{
	"codex":    ".codex/skills/andurel",
	"claude":   ".claude/skills/andurel",
	"pi":       ".pi/skills/andurel",
	"opencode": ".opencode/skills/andurel",
	"crush":    ".crush/skills/andurel",
}

func runSkillCommandTest(t *testing.T, input string, args ...string) skillCommandTestResult {
	t.Helper()
	resetCLITestSeams(t)

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	findGoModRoot = func() (string, error) { return rootDir, nil }

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand("test", "test-date")
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return skillCommandTestResult{
		rootDir: rootDir,
		stdout:  stdout.String(),
		stderr:  stderr.String(),
		err:     err,
	}
}

func assertInstalledSkill(t *testing.T, rootDir, directory string) {
	t.Helper()
	if err := skills.WalkAndurelSkillFiles(func(path string, expected []byte) error {
		installed, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(directory), filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if !bytes.Equal(installed, expected) {
			return fmt.Errorf("installed %s does not match embedded asset", path)
		}
		return nil
	}); err != nil {
		t.Fatalf("verify installed skill at %s: %v", directory, err)
	}
}

func assertSkillNotInstalled(t *testing.T, rootDir, directory string) {
	t.Helper()
	path := filepath.Join(rootDir, filepath.FromSlash(directory))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unselected skill path %s exists or could not be inspected: %v", path, err)
	}
}

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
)

func TestNewProjectHarnessSelection(t *testing.T) {
	t.Run("unknown harness is a usage error before scaffolding", func(t *testing.T) {
		cmd := newProjectCommand("test")

		err := newProject(cmd, []string{"app"}, "test", true, false, []string{"definitely-not-a-harness"})
		if err == nil || !strings.Contains(err.Error(), "unknown harness") {
			t.Fatalf("unknown harness error = %v", err)
		}
		var cliErr *output.CLIError
		if !errors.As(err, &cliErr) {
			t.Fatalf("error = %T, want *output.CLIError", err)
		}
		if !strings.Contains(cliErr.Hint, "codex, claude, pi, opencode, crush") {
			t.Fatalf("unknown harness hint does not list the harnesses: %q", cliErr.Hint)
		}
	})

	t.Run("comma-separated and repeated harnesses are deduplicated", func(t *testing.T) {
		cmd := newProjectCommand("test")
		if err := cmd.Flags().Set("harness", "claude,pi"); err != nil {
			t.Fatalf("set harness: %v", err)
		}
		if err := cmd.Flags().Set("harness", "claude"); err != nil {
			t.Fatalf("set harness again: %v", err)
		}
		requested, err := cmd.Flags().GetStringSlice("harness")
		if err != nil {
			t.Fatalf("read harness flag: %v", err)
		}

		harnesses, err := parseSkillHarnessNames(requested)
		if err != nil {
			t.Fatalf("parse harnesses: %v", err)
		}
		if len(harnesses) != 2 || harnesses[0].name != "claude" || harnesses[1].name != "pi" {
			t.Fatalf("harnesses = %#v", harnesses)
		}
	})
}

func TestInstallNewProjectSkillsWritesSelectedHarnessesOnly(t *testing.T) {
	root := t.TempDir()
	harnesses, err := parseSkillHarnessNames([]string{"claude"})
	if err != nil {
		t.Fatalf("parse harnesses: %v", err)
	}

	if err := installNewProjectSkills(root, harnesses); err != nil {
		t.Fatalf("install skills: %v", err)
	}
	// A second run must be idempotent: andurel new --harness can be repeated.
	if err := installNewProjectSkills(root, harnesses); err != nil {
		t.Fatalf("install skills twice: %v", err)
	}

	skillPath := filepath.Join(root, ".claude", "skills", "andurel", "SKILL.md")
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("installed skill is empty")
	}
	for _, harness := range skillHarnesses {
		if harness.name == "claude" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, harness.directory)); err == nil {
			t.Fatalf("unselected harness %s was installed", harness.name)
		}
	}
}

func TestNewProjectBreadcrumbsNameTheSkillInstallExample(t *testing.T) {
	withoutSkill := newProjectBreadcrumbs(true, "pnpm", nil)
	var found bool
	for _, breadcrumb := range withoutSkill {
		if breadcrumb.Command == "andurel skill install --harness claude" {
			found = true
			if !strings.Contains(breadcrumb.Description, "codex, claude, pi, opencode, crush") {
				t.Fatalf("skill breadcrumb description = %q", breadcrumb.Description)
			}
		}
	}
	if !found {
		t.Fatalf("breadcrumbs do not name an explicit skill install: %#v", withoutSkill)
	}

	withSkill := newProjectBreadcrumbs(true, "pnpm", []skillHarness{
		{name: "claude", label: "Claude", directory: ".claude/skills/andurel"},
	})
	for _, breadcrumb := range withSkill {
		if strings.Contains(breadcrumb.Command, "skill install") {
			t.Fatalf("breadcrumbs still ask to install an installed skill: %#v", breadcrumb)
		}
	}
}

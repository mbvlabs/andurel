package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/skills"
	"github.com/spf13/cobra"
)

type skillReport struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	Body string `json:"body,omitempty"`
}

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Show or install the embedded Andurel agent skill",
		Long:  "Show or install the embedded Andurel agent skill with command recipes and invariants.",
	}
	setAgentMetadata(cmd, "skill", "Provides the embedded Andurel agent skill.")

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the embedded Andurel skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeJSON || opts.Mode == output.ModeAgent {
				return output.OK(cmd, skillReport{Name: "andurel", Body: skills.AndurelSkill}, "Loaded Andurel skill")
			}
			if opts.Quiet {
				return nil
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), skills.AndurelSkill); err != nil {
				return err
			}
			if !strings.HasSuffix(skills.AndurelSkill, "\n") {
				_, err = fmt.Fprintln(cmd.OutOrStdout())
				return err
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install the embedded Andurel skill into the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := projectCodexSkillPath()
			if err != nil {
				return err
			}
			if err := installAndurelSkill(filepath.Dir(target)); err != nil {
				return err
			}
			return output.OK(cmd, skillReport{Name: "andurel", Path: target}, "Installed Andurel skill")
		},
	})

	return cmd
}

func projectCodexSkillPath() (string, error) {
	rootDir, err := findGoModRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootDir, ".codex", "skills", "andurel", "SKILL.md"), nil
}

func installAndurelSkill(targetDir string) error {
	return skills.WalkAndurelSkillFiles(func(path string, data []byte) error {
		target := filepath.Join(targetDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

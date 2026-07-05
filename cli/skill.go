package cli

import (
	"os"
	"path/filepath"

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
			return output.OK(cmd, skillReport{Name: "andurel", Body: skills.AndurelSkill}, "Loaded Andurel skill")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install the embedded Andurel skill into CODEX_HOME",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := codexSkillPath()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte(skills.AndurelSkill), 0o644); err != nil {
				return err
			}
			return output.OK(cmd, skillReport{Name: "andurel", Path: target}, "Installed Andurel skill")
		},
	})

	return cmd
}

func codexSkillPath() (string, error) {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(userHome, ".codex")
	}
	return filepath.Join(home, "skills", "andurel", "SKILL.md"), nil
}

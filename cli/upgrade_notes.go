package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
	"github.com/spf13/cobra"
)

func newUpgradeNotesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-notes",
		Short: "Show the manual actions of a past upgrade",
		Long: `Show the manual actions an upgrade from --from to the project's current
version prints. After an upgrade andurel.toml records the new version, so
andurel upgrade no longer lists them; this command shows them again.

The actions cover application-owned code that Andurel does not rewrite. Each
one says how to tell whether a fix you wrote yourself is equivalent.`,
		Example: `  andurel inspect upgrade-notes --from v2.0.0-beta.2
  andurel inspect upgrade-notes --from v2.0.0-beta.2 --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgradeNotes(cmd)
		},
	}
	cmd.Flags().String("from", "", "Version the project was upgraded from (required)")
	setAgentMetadata(
		cmd,
		"maintenance",
		"Read-only list of the manual actions between --from and the version in andurel.toml.",
	)
	return cmd
}

type upgradeNotesReport struct {
	FromVersion   string                 `json:"from_version"`
	ToVersion     string                 `json:"to_version"`
	ManualActions []upgrade.ManualAction `json:"manual_actions"`
}

func runUpgradeNotes(cmd *cobra.Command) error {
	from, err := cmd.Flags().GetString("from")
	if err != nil {
		return err
	}
	from = strings.TrimSpace(from)
	if from == "" {
		return output.NewError(
			output.CodeUsage,
			"--from is required",
			output.ExitUsage,
			"Pass the version the project was upgraded from, e.g. andurel inspect upgrade-notes --from v2.0.0-beta.2.",
		)
	}

	projectRoot, err := findGoModRoot()
	if err != nil {
		return err
	}
	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return output.WrapError(
			output.CodeConfigError,
			fmt.Errorf("read andurel.toml: %w", err),
			output.ExitConfig,
			"Run this command from an Andurel project root.",
		)
	}

	inertia := lock.ScaffoldConfig != nil &&
		layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia)
	actions, err := upgrade.ManualActionsBetween(from, lock.Version, inertia)
	if err != nil {
		return output.WrapError(
			output.CodeUsage,
			err,
			output.ExitUsage,
			fmt.Sprintf("Pass a version older than the project's %s.", lock.Version),
		)
	}

	report := upgradeNotesReport{
		FromVersion:   from,
		ToVersion:     lock.Version,
		ManualActions: actions,
	}
	summary := fmt.Sprintf("%d manual actions from %s to %s", len(actions), from, lock.Version)

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if output.UsesStructuredOutput(opts) {
		return output.OK(cmd, report, summary)
	}
	if opts.Quiet {
		return nil
	}

	out := cmd.OutOrStdout()
	if len(actions) == 0 {
		_, err := fmt.Fprintf(out, "No manual actions from %s to %s.\n", from, lock.Version)
		return err
	}
	if _, err := fmt.Fprintf(out, "Manual actions from %s to %s:\n", from, lock.Version); err != nil {
		return err
	}
	for _, action := range actions {
		if _, err := fmt.Fprintf(
			out,
			"\n%s (%s)\n\n%s\n",
			action.Title,
			action.ID,
			strings.TrimSpace(action.Instructions),
		); err != nil {
			return err
		}
	}
	return nil
}

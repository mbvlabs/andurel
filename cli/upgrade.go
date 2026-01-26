package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/layout/upgrade"
	"github.com/spf13/cobra"
)

func newUpgradeCommand(version string) *cobra.Command {
	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade framework files to latest version",
		Long: `Upgrade framework files and tool versions.

⚠️  IMPORTANT: Commit or create a branch before upgrading! This command modifies files in place.

This command will:
  1. Replace framework-managed files with the latest version
  2. Update tool versions in andurel.lock

Note: This only upgrades framework code. You are responsible for updating
your application code to work with any API changes in the new version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, version)
		},
	}

	upgradeCmd.Flags().Bool("dry-run", false, "Show what would be changed without applying")

	return upgradeCmd
}

func runUpgrade(cmd *cobra.Command, targetVersion string) error {
	projectRoot, err := findGoModRoot()
	if err != nil {
		return err
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}

	opts := upgrade.UpgradeOptions{
		DryRun:        dryRun,
		Auto:          false,
		TargetVersion: targetVersion,
	}

	fmt.Printf("Upgrading project to version %s...\n\n", targetVersion)

	upgrader, err := upgrade.NewUpgrader(projectRoot, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize upgrader: %w", err)
	}

	report, err := upgrader.Execute()
	if err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	if report.Success {
		fmt.Printf("\n✓ Upgrade complete!\n")

		// Sync tools if any were added, updated, or removed
		totalToolChanges := report.ToolsAdded + report.ToolsUpdated + report.ToolsRemoved
		if totalToolChanges > 0 {
			fmt.Printf("\nSyncing tools...\n")
			if err := syncBinaries(projectRoot); err != nil {
				fmt.Printf("⚠ Warning: failed to sync tools: %v\n", err)
				fmt.Printf("You can manually sync tools by running: andurel sync\n")
			}
		}

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  1. Review the changes with 'git diff'\n")
		fmt.Printf("  2. Update your application code if needed for API changes\n")
		fmt.Printf("  3. Test your application\n")
		fmt.Printf("  4. Commit when ready\n")
		return nil
	}

	return nil
}

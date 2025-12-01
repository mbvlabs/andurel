package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/upgrade"
	"github.com/spf13/cobra"
)

type upgradeState struct {
	FromVersion   string   `json:"from_version"`
	ToVersion     string   `json:"to_version"`
	ConflictFiles []string `json:"conflict_files"`
	InProgress    bool     `json:"in_progress"`
}

const upgradeStateFile = ".andurel-upgrade-state.json"

func newUpgradeCommand(version string) *cobra.Command {
	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade project to latest Andurel templates",
		Long: `Upgrade an existing Andurel project to use the latest framework templates.

⚠️  IMPORTANT: Commit or create a branch before upgrading! This command modifies files in place.

This command will:
  1. Generate fresh templates using the latest version
  2. Intelligently merge changes while preserving your modifications
  3. Mark any conflicts for manual review

After upgrade with conflicts, run 'andurel upgrade finalize' when resolved.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, version)
		},
	}

	upgradeCmd.Flags().Bool("dry-run", false, "Show what would be changed without applying")
	upgradeCmd.Flags().Bool("auto", false, "Apply all safe changes without prompting")

	upgradeCmd.AddCommand(newUpgradeFinalizeCommand())
	upgradeCmd.AddCommand(newUpgradeStatusCommand())

	return upgradeCmd
}

func runUpgrade(cmd *cobra.Command, targetVersion string) error {
	projectRoot, err := findGoModRoot()
	if err != nil {
		return err
	}

	state, err := loadUpgradeState(projectRoot)
	if err == nil && state.InProgress {
		return fmt.Errorf(
			"upgrade already in progress from %s to %s\nResolve conflicts and run 'andurel upgrade finalize' or 'andurel upgrade abort'",
			state.FromVersion,
			state.ToVersion,
		)
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}

	// auto, err := cmd.Flags().GetBool("auto")
	// if err != nil {
	// 	return err
	// }

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

	if len(report.ConflictFiles) > 0 {
		state := &upgradeState{
			FromVersion:   report.FromVersion,
			ToVersion:     report.ToVersion,
			ConflictFiles: report.ConflictFiles,
			InProgress:    true,
		}

		if err := saveUpgradeState(projectRoot, state); err != nil {
			fmt.Printf("Warning: failed to save upgrade state: %v\n", err)
		}

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  1. Review and resolve conflicts in the files above\n")
		fmt.Printf("  2. Run 'andurel upgrade finalize' to complete the upgrade\n")

		return nil
	}

	if report.Success {
		fmt.Printf("\n✓ All changes applied successfully!\n")
		return nil
	}

	return nil
}

func newUpgradeFinalizeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "finalize",
		Short: "Finalize upgrade after resolving conflicts",
		Long: `Complete the upgrade process after manually resolving any conflicts.

This command will:
  1. Verify all conflicts have been resolved
  2. Clean up upgrade state`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgradeFinalize()
		},
	}
}

func runUpgradeFinalize() error {
	projectRoot, err := findGoModRoot()
	if err != nil {
		return err
	}

	state, err := loadUpgradeState(projectRoot)
	if err != nil {
		return fmt.Errorf("no upgrade in progress")
	}

	if !state.InProgress {
		return fmt.Errorf("no upgrade in progress")
	}

	fmt.Printf("Finalizing upgrade from %s to %s...\n\n", state.FromVersion, state.ToVersion)

	for _, file := range state.ConflictFiles {
		filePath := filepath.Join(projectRoot, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		if hasConflictMarkers(content) {
			return fmt.Errorf(
				"file %s still contains conflict markers\nPlease resolve all conflicts before finalizing",
				file,
			)
		}
	}

	if err := removeUpgradeState(projectRoot); err != nil {
		fmt.Printf("Warning: failed to clean up upgrade state: %v\n", err)
	}

	fmt.Printf("✓ Upgrade finalized successfully!\n")
	fmt.Printf("\nYour project is now at version %s\n", state.ToVersion)
	fmt.Printf("\nRemember to commit your changes when ready.\n")

	return nil
}

func newUpgradeStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current upgrade status",
		Long:  `Display information about the current upgrade state, if any.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgradeStatus()
		},
	}
}

func runUpgradeStatus() error {
	projectRoot, err := findGoModRoot()
	if err != nil {
		return err
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	fmt.Printf("Current project version: %s\n", lock.TemplateVersion)

	state, err := loadUpgradeState(projectRoot)
	if err != nil {
		fmt.Printf("No upgrade in progress\n")
		return nil
	}

	if !state.InProgress {
		fmt.Printf("No upgrade in progress\n")
		return nil
	}

	fmt.Printf("\n⚠ Upgrade in progress\n")
	fmt.Printf("From version: %s\n", state.FromVersion)
	fmt.Printf("To version: %s\n", state.ToVersion)

	if len(state.ConflictFiles) > 0 {
		fmt.Printf("\nFiles with conflicts (%d):\n", len(state.ConflictFiles))
		for _, file := range state.ConflictFiles {
			fmt.Printf("  - %s\n", file)
		}

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  1. Resolve conflicts in the files above\n")
		fmt.Printf("  2. Run 'andurel upgrade finalize' to complete\n")
	}

	return nil
}

func loadUpgradeState(projectRoot string) (*upgradeState, error) {
	statePath := filepath.Join(projectRoot, upgradeStateFile)

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}

	var state upgradeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func saveUpgradeState(projectRoot string, state *upgradeState) error {
	statePath := filepath.Join(projectRoot, upgradeStateFile)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0o644)
}

func removeUpgradeState(projectRoot string) error {
	statePath := filepath.Join(projectRoot, upgradeStateFile)
	return os.Remove(statePath)
}

func hasConflictMarkers(content []byte) bool {
	markers := []string{
		"<<<<<<<",
		"=======",
		">>>>>>>",
	}

	contentStr := string(content)
	for _, marker := range markers {
		if len(contentStr) >= len(marker) {
			for i := 0; i <= len(contentStr)-len(marker); i++ {
				if contentStr[i:i+len(marker)] == marker {
					return true
				}
			}
		}
	}

	return false
}

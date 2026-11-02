package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
	"github.com/spf13/cobra"
)

func newUpgradeCommand(version string) *cobra.Command {
	upgradeCmd := &cobra.Command{
		Use:     "upgrade",
		Aliases: []string{"up"},
		Short:   "Upgrade framework files to latest version",
		Long: `Upgrade framework files and tool versions.

⚠️  IMPORTANT: Commit or create a branch before upgrading! This command modifies files in place.

This command will:
  1. Verify that the installed Andurel CLI is the latest release
  2. Replace framework-managed files with the latest version
  3. Update tool versions in andurel.lock
  4. Pin required github.com/mbvlabs/andurel/pkg/* modules in go.mod to the
     versions verified with this CLI (never downgrades a newer pin)

You are responsible for updating application code to work with any API
changes. After go.mod changes, run go mod tidy. To take the latest published
package versions without a framework upgrade, use andurel packages update.

Structured output (--json/--agent) is non-interactive: it never runs go mod
tidy or andurel tool sync. commands_run records the commands that actually
executed, including read-only git checks; failed attempts carry a (failed)
suffix. ok:true means the upgrade mutation succeeded, not that the project
builds. Read warnings and breadcrumbs for required follow-up work.`,
		Example: `  andurel upgrade
  andurel upgrade --dry-run
  andurel upgrade --repair`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return upgradeCommandError(runUpgrade(cmd, version))
		},
	}

	upgradeCmd.Flags().Bool("dry-run", false, "Show what would be changed without applying")
	upgradeCmd.Flags().Bool("diff", false, "Include a text diff preview in structured output")
	upgradeCmd.Flags().
		Bool("repair", false, "Restore drifted framework-owned files when already on this version")

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
	diff, err := cmd.Flags().GetBool("diff")
	if err != nil {
		return err
	}
	outOpts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	repair := false
	if cmd.Flags().Lookup("repair") != nil {
		repair, err = cmd.Flags().GetBool("repair")
		if err != nil {
			return err
		}
	}
	structured := output.SuppressesHumanOutput(outOpts)
	if !repair {
		if err := requireLatestAndurelRelease(cmd.Context(), targetVersion); err != nil {
			return err
		}
	}

	opts := upgrade.UpgradeOptions{
		DryRun: dryRun,
		// In structured mode there is no TTY to confirm on, so an explicit
		// --repair runs non-interactively (Auto). Human mode keeps the
		// confirm prompt below unless --dry-run.
		Auto:          structured && repair && !dryRun,
		Repair:        repair && !dryRun,
		TargetVersion: targetVersion,
	}

	upgrader, err := newUpgraderFunc(projectRoot, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize upgrader: %w", err)
	}

	before, snapErr := snapshotFilesForReport(projectRoot)
	if snapErr != nil {
		return snapErr
	}
	report, err := func() (*upgrade.UpgradeReport, error) {
		if structured {
			var report *upgrade.UpgradeReport
			runErr := runWithOptionalStdoutSilence(true, func() error {
				var executeErr error
				report, executeErr = upgrader.Execute()
				return executeErr
			})
			return report, runErr
		}
		return upgrader.Execute()
	}()
	if err != nil {
		return upgradeCommandErrorWithReport(err, report, structured)
	}

	if structured {
		after, err := snapshotFilesForReport(projectRoot)
		if err != nil {
			return err
		}
		if dryRun && report != nil {
			// Nothing was written, so project the plan onto the snapshot. The
			// dry run then reports the same files and diff as the real run,
			// andurel.toml and andurel.lock included.
			after = applyPlannedChanges(before, report.PlannedChanges)
		}
		followUpWarnings, followUpBreadcrumbs := upgradeFollowUps(report, dryRun)
		if dryRun {
			followUpWarnings = append(
				[]string{"dry run only; no files were changed"},
				followUpWarnings...,
			)
		}
		artifactReport := buildMutationReport(mutationOptions{
			Action:   "upgrade",
			Resource: targetVersion,
			DryRun:   dryRun,
			Diff:     diff,
			Warnings: followUpWarnings,
		}, before, after)
		// commands_run mirrors the subprocesses the upgrade domain actually
		// attempted. It never includes proposed follow-up work.
		artifactReport.CommandsRun = []string{}
		if report != nil {
			artifactReport.CommandsRun = append([]string{}, report.CommandsRun...)
		}
		data := map[string]any{
			"upgrade":   report,
			"artifacts": artifactReport,
		}
		breadcrumbs := []output.Breadcrumb{}
		if report != nil && report.RepairAvailable && !report.RepairApplied {
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "andurel upgrade --repair",
				Description: "Restore framework-owned drift; file and tool changes in this report are proposed until this runs",
			})
		}
		breadcrumbs = append(breadcrumbs,
			output.Breadcrumb{Command: "andurel doctor"},
			output.Breadcrumb{Command: "git diff"},
		)
		breadcrumbs = append(breadcrumbs, followUpBreadcrumbs...)
		return output.OK(
			cmd,
			data,
			mutationSummary(artifactReport),
			breadcrumbs...,
		)
	}

	if report.AlreadyCurrent {
		return nil
	}
	if report.RepairAvailable && !report.RepairApplied {
		if dryRun {
			return nil
		}
		if report.DirtyWorktree {
			return fmt.Errorf(
				"worktree is dirty; commit or stash changes before repairing framework files",
			)
		}
		confirmed, err := confirmFrameworkRepair(cmd)
		if err != nil {
			return err
		}
		if !confirmed {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No files were changed.")
			return nil
		}
		opts.Repair = true
		opts.Auto = true
		upgrader, err = newUpgraderFunc(projectRoot, opts)
		if err != nil {
			return fmt.Errorf("failed to initialize framework repair: %w", err)
		}
		report, err = upgrader.Execute()
		if err != nil {
			return upgradeCommandErrorWithReport(err, report, structured)
		}
	}

	if dryRun {
		return nil
	}

	if report.Success {
		if report.RepairApplied {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Framework repair complete for version %s\n", targetVersion)
			printUpgradeSummary(cmd, report)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  1. Review the restored files with 'git diff'")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  2. Test your application")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  3. Commit when ready")
			return nil
		}
		// Sync tools if any were added, updated, or removed
		totalToolChanges := report.ToolsAdded + report.ToolsUpdated + report.ToolsRemoved
		if totalToolChanges > 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nSyncing tool binaries...")
			if err := syncBinaries(projectRoot); err != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "⚠ Warning: failed to sync tools: %v\n", err)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "You can manually sync tools by running: andurel tool sync")
			}
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Upgrade complete! Project is now at version %s\n", targetVersion)
		printUpgradeSummary(cmd, report)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  1. Review the changes with 'git diff'")
		step := 2
		if slices.Contains(report.ReplacedFiles, "go.mod") {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. Run 'go mod tidy' to refresh go.sum\n", step)
			step++
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. Update your application code if needed for API changes\n", step)
		step++
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. Test your application\n", step)
		step++
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. Commit when ready\n", step)
		return nil
	}

	return nil
}

// upgradeCommandError maps upgrade-domain typed errors onto the shared CLI
// error contract. Without this the structured JSON envelope would only see a
// generic "error" and the unsupported_upgrade code, hint and data would be
// lost.
func upgradeCommandError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*output.CLIError](err); ok {
		return err
	}
	var unsupported *upgrade.UnsupportedUpgradeError
	if !errors.As(err, &unsupported) {
		return err
	}
	return &output.CLIError{
		Code:     output.CodeUnsupportedUpgrade,
		Message:  unsupported.Error(),
		Hint:     unsupported.Hint(),
		ExitCode: output.ExitConfig,
		Cause:    err,
		Data:     unsupported.Data(),
	}
}

// upgradeCommandErrorWithReport keeps the typed error contract and adds the
// failed execution's report to structured error data. The command still
// returns an error envelope, never a success envelope.
func upgradeCommandErrorWithReport(
	err error,
	report *upgrade.UpgradeReport,
	structured bool,
) error {
	mapped := upgradeCommandError(err)
	if !structured || report == nil {
		return mapped
	}

	if cliErr, ok := errors.AsType[*output.CLIError](mapped); ok {
		cloned := *cliErr
		cloned.Data = mergeUpgradeErrorData(cliErr.Data, report)
		return &cloned
	}

	envelope := output.Fail(mapped)
	return &output.CLIError{
		Code:     envelope.Code,
		Message:  envelope.Error,
		Hint:     envelope.Hint,
		ExitCode: envelope.ExitCode,
		Cause:    mapped,
		Data:     mergeUpgradeErrorData(nil, report),
	}
}

// mergeUpgradeErrorData preserves existing typed data and adds the failed
// report plus its actual command provenance.
func mergeUpgradeErrorData(existing any, report *upgrade.UpgradeReport) map[string]any {
	data := map[string]any{}
	switch value := existing.(type) {
	case nil:
	case map[string]any:
		maps.Copy(data, value)
	case upgrade.UnsupportedUpgradeData:
		data["from"] = value.From
		data["to"] = value.To
		data["rebuild_version"] = value.RebuildVersion
		data["legacy_markers"] = value.LegacyMarkers
	default:
		encoded, err := json.Marshal(value)
		if err == nil {
			_ = json.Unmarshal(encoded, &data)
		}
	}
	data["upgrade"] = report
	data["commands_run"] = append([]string{}, report.CommandsRun...)
	return data
}

func applyPlannedChanges(before fileSnapshot, changes []upgrade.PlannedChange) fileSnapshot {
	after := make(fileSnapshot, len(before))
	maps.Copy(after, before)
	for _, change := range changes {
		if change.Remove {
			delete(after, change.Path)
			continue
		}
		after[change.Path] = fileState{
			Hash:    sha256.Sum256(change.Content),
			Content: change.Content,
			Mode:    change.Mode,
		}
	}
	return after
}

// upgradeFollowUps reports the follow-up work a completed or pending mutation
// actually needs. Structured upgrades are non-interactive: they never run
// go mod tidy or andurel tool sync, so the guidance is returned as warnings
// and breadcrumbs rather than command records. A dry run or pending repair
// phrases the work as a proposal because nothing has changed yet.
func upgradeFollowUps(
	report *upgrade.UpgradeReport,
	dryRun bool,
) ([]string, []output.Breadcrumb) {
	if report == nil {
		return nil, nil
	}

	pendingRepair := report.RepairAvailable && !report.RepairApplied
	var warnings []string
	var breadcrumbs []output.Breadcrumb
	if pendingRepair {
		warnings = append(
			warnings,
			"framework-owned file drift detected; run 'andurel upgrade --repair' first. File and tool changes in this report are proposed, not applied.",
		)
	}

	if slices.Contains(report.ReplacedFiles, "go.mod") {
		switch {
		case pendingRepair:
			warnings = append(
				warnings,
				"pending repair: the go.mod change is proposed, not applied; after 'andurel upgrade --repair', 'go mod tidy' is proposed to refresh go.sum; it was not run",
			)
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "go mod tidy",
				Description: "Proposed after the pending repair: refresh go.sum for repaired go.mod pins",
			})
		case dryRun:
			warnings = append(
				warnings,
				"dry run: 'go mod tidy' is proposed after a real upgrade to refresh go.sum for updated go.mod pins; it was not run",
			)
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "go mod tidy",
				Description: "Proposed after a real upgrade: refresh go.sum for updated go.mod pins",
			})
		default:
			warnings = append(
				warnings,
				"go.mod pins changed; run 'go mod tidy' to refresh go.sum",
			)
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "go mod tidy",
				Description: "Required after this upgrade: refresh go.sum for updated go.mod pins",
			})
		}
	}

	if report.ToolsAdded+report.ToolsUpdated+report.ToolsRemoved > 0 {
		switch {
		case pendingRepair:
			warnings = append(
				warnings,
				"pending repair: tool version changes are proposed, not applied; after 'andurel upgrade --repair', 'andurel tool sync' is proposed to install them; it was not run",
			)
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "andurel tool sync",
				Description: "Proposed after the pending repair: install changed tool binaries",
			})
		case dryRun:
			warnings = append(
				warnings,
				"dry run: 'andurel tool sync' is proposed after a real upgrade to install changed tool binaries; it was not run",
			)
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "andurel tool sync",
				Description: "Proposed after a real upgrade: install changed tool binaries",
			})
		default:
			warnings = append(
				warnings,
				"tool versions changed; run 'andurel tool sync' to install the pinned binaries",
			)
			breadcrumbs = append(breadcrumbs, output.Breadcrumb{
				Command:     "andurel tool sync",
				Description: "Required after this upgrade: install changed tool binaries",
			})
		}
	}

	return warnings, breadcrumbs
}

func requireLatestAndurelRelease(ctx context.Context, currentVersion string) error {
	current, ok := canonicalAndurelVersion(currentVersion)
	if !ok {
		return nil
	}

	latest, err := lookupLatestAndurelVersionFunc(ctx)
	if err != nil || !newerAndurelVersion(current, latest) {
		return nil
	}

	return output.NewError(
		output.CodeUpdateRequired,
		fmt.Sprintf(
			"a newer Andurel CLI is required to upgrade this project (current: %s, latest: %s)",
			current,
			latest,
		),
		output.ExitDependency,
		fmt.Sprintf("Run '%s', then run 'andurel upgrade' again.", andurelInstallCommand(latest)),
	)
}

func confirmFrameworkRepair(cmd *cobra.Command) (bool, error) {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Restore these framework-owned files? [y/N] ")
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func printUpgradeSummary(cmd *cobra.Command, report *upgrade.UpgradeReport) {
	if report.FilesReplaced == 0 && report.FilesRemoved == 0 && report.ToolsAdded == 0 &&
		report.ToolsUpdated == 0 && report.ToolsRemoved == 0 && len(report.ToolMetadataChanges) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, "\nSummary:")
	if report.FilesReplaced > 0 {
		_, _ = fmt.Fprintf(out, "  • %d framework files replaced\n", report.FilesReplaced)
	}
	if report.FilesRemoved > 0 {
		_, _ = fmt.Fprintf(out, "  • %d obsolete internal package files removed\n", report.FilesRemoved)
	}
	if report.ToolsAdded > 0 {
		_, _ = fmt.Fprintf(out, "  • %d tools added\n", report.ToolsAdded)
	}
	if report.ToolsUpdated > 0 {
		_, _ = fmt.Fprintf(out, "  • %d tools updated\n", report.ToolsUpdated)
	}
	if report.ToolsRemoved > 0 {
		_, _ = fmt.Fprintf(out, "  • %d tools removed\n", report.ToolsRemoved)
	}
	if len(report.ToolMetadataChanges) > 0 {
		_, _ = fmt.Fprintf(out, "  • %d tool metadata entries updated\n", len(report.ToolMetadataChanges))
	}
}

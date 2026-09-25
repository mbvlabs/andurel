package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/generator"
	"github.com/spf13/cobra"
)

type modelSyncReport struct {
	Path    string `json:"path"`
	Stale   bool   `json:"stale"`
	Written bool   `json:"written"`
	Diff    string `json:"diff,omitempty"`
}

func newSyncModelCommand() *cobra.Command {
	var check bool
	var diff bool

	cmd := &cobra.Command{
		Use:   "model NAME",
		Short: "Sync a model from migrations",
		Long: `Refresh an existing model file from the current SQL migrations.

Writes by default. Use --check to report drift without writing, and --diff to
include the proposed model changes. Does not sync factories — use
andurel sync factory NAME for that. Custom models (// andurel:custom) cannot
be synced from migrations.`,
		Example: `  andurel sync model Post
  andurel sync model Post --check
  andurel sync model Post --check --diff`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelSyncCommand(cmd, args[0], check, diff)
		},
	}
	setAgentMetadata(
		cmd,
		"generation",
		"Syncs one model from migrations. Use --check --json in CI; bare sync model NAME writes.",
	)

	cmd.Flags().BoolVar(&check, "check", false, "Report model drift without writing")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include proposed model changes")

	return cmd
}

func runModelSyncCommand(cmd *cobra.Command, resourceName string, check, includeDiff bool) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	gen, err := newGenerator()
	if err != nil {
		return err
	}

	result, err := gen.UpdateModel(resourceName)
	if err != nil {
		return err
	}
	clearFactoryFromModelUpdate(result)

	report, err := buildModelSyncReport(result, includeDiff)
	if err != nil {
		return err
	}

	if !result.HasChanges {
		return renderModelSyncReport(cmd, report, "No changes — model is already up to date.")
	}

	if check {
		outOpts, parseErr := output.ParseOptions(cmd)
		if parseErr != nil {
			return parseErr
		}
		checkErr := output.NewError(
			output.CodeGenerationFailed,
			fmt.Sprintf("model %s is stale", resourceName),
			output.ExitGeneration,
			fmt.Sprintf("Run andurel sync model %s.", resourceName),
		)
		checkErr.Data = report
		if output.UsesStructuredOutput(outOpts) {
			return checkErr
		}
		if renderErr := printModelSyncHuman(cmd, report); renderErr != nil {
			return renderErr
		}
		return checkErr
	}

	if err := gen.ApplyModelUpdate(result); err != nil {
		return err
	}
	report.Written = true
	report.Stale = false
	return renderModelSyncReport(cmd, report, fmt.Sprintf("Updated %s", result.ModelPath))
}

func clearFactoryFromModelUpdate(result *generator.UpdateModelResult) {
	if result == nil {
		return
	}
	result.FactoryPath = ""
	result.OldFactoryContent = ""
	result.NewFactoryContent = ""
	result.FactoryHasChanges = false
}

func buildModelSyncReport(
	result *generator.UpdateModelResult,
	includeDiff bool,
) (modelSyncReport, error) {
	report := modelSyncReport{
		Path:  result.ModelPath,
		Stale: result.HasChanges,
	}
	if includeDiff && result.HasChanges {
		diff, err := result.Diff()
		if err != nil {
			return modelSyncReport{}, fmt.Errorf("failed to compute diff: %w", err)
		}
		report.Diff = diff
	}
	return report, nil
}

func renderModelSyncReport(cmd *cobra.Command, report modelSyncReport, summary string) error {
	outOpts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if output.UsesStructuredOutput(outOpts) {
		return output.OK(cmd, report, summary)
	}
	if outOpts.Quiet {
		return nil
	}
	return printModelSyncHuman(cmd, report)
}

func printModelSyncHuman(cmd *cobra.Command, report modelSyncReport) error {
	switch {
	case report.Written:
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", report.Path); err != nil {
			return err
		}
	case report.Stale:
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Stale %s\n", report.Path); err != nil {
			return err
		}
	default:
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes — model is already up to date."); err != nil {
			return err
		}
	}
	if strings.TrimSpace(report.Diff) != "" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
			return err
		}
		printColoredDiff(report.Diff)
	}
	return nil
}

func printColoredDiff(diff string) {
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			fmt.Println(line)
		case strings.HasPrefix(line, "+"):
			fmt.Printf("\033[32m%s\033[0m\n", line)
		case strings.HasPrefix(line, "-"):
			fmt.Printf("\033[31m%s\033[0m\n", line)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("\033[36m%s\033[0m\n", line)
		default:
			fmt.Println(line)
		}
	}
}

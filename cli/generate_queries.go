package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout/cmds"
	"github.com/spf13/cobra"
)

const queryManifestWarning = "Query generation may change shared go.mod and go.sum; coordinate with other writers and review manifest changes. Use --dry-run to preview."

var generateQueriesFunc = generateNarsilcIfNeeded

func newGenerateQueriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "queries",
		Aliases: []string{"q"},
		Short:   "Generate Go code from narsilc SQL queries",
		Long: `Run narsilc generate for SQL files under domains/.

Generated code is written to domains/internal/queries. Generation may also change
shared go.mod and go.sum. Use --dry-run to preview changes in a temporary copy;
coordinate with other writers before applying. A preview is not a lock and may
become stale. This command is a no-op when no annotated SQL files exist.`,
		Example: `  andurel sync queries --dry-run --json
  andurel sync queries`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			hasQueries, err := cmds.HasQueryFiles(rootDir)
			if err != nil {
				return fmt.Errorf("check narsilc queries: %w", err)
			}
			commandsRun := []string{}
			warnings := []string{queryManifestWarning}
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}
			if hasQueries {
				commandsRun = append(
					commandsRun,
					"narsilc generate",
					"go fmt ./domains/internal/queries/...",
				)
			} else {
				warnings = append(
					warnings,
					"no annotated narsilc query files found under domains/; generation was skipped",
				)
			}

			outOpts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if !output.SuppressesHumanOutput(outOpts) && !outOpts.Quiet {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), queryManifestWarning); err != nil {
					return err
				}
			}
			err = runMutation(cmd, mutationOptions{
				Action:      "generate queries",
				RootDir:     rootDir,
				DryRun:      dryRun,
				CommandsRun: commandsRun,
				Warnings:    warnings,
				Breadcrumbs: []output.Breadcrumb{
					{
						Command:     "andurel doctor --json",
						Description: "Check generated narsilc code for drift",
					},
				},
				Run: func(rootDir string) error {
					if !hasQueries {
						fmt.Println(
							"No annotated narsilc query files found under domains/; skipping generation.",
						)
						return nil
					}
					if err := generateQueriesFunc(rootDir); err != nil {
						return err
					}
					if dryRun {
						fmt.Println("Generated preview in temporary copy: domains/internal/queries")
					} else {
						fmt.Println("✓ Generated typed queries: domains/internal/queries")
					}
					return nil
				},
			})
			if err != nil && hasQueries && !dryRun {
				return fmt.Errorf("query generation failed; files including shared go.mod and go.sum may have been partially changed: %w", err)
			}
			return err
		},
	}
	cmd.Flags().Bool("dry-run", false, "Preview query and shared manifest changes in a temporary copy")
	setAgentMetadata(
		cmd,
		"generation",
		"Runs only for annotated SQL. May change shared go.mod and go.sum; use --dry-run to preview. Returns a mutation report in structured modes.",
	)
	return cmd
}

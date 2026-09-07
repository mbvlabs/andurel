package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/spf13/cobra"
)

func newGenerateQueriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "queries",
		Aliases: []string{"q"},
		Short:   "Generate Go code from sqlc SQL queries",
		Long: `Run sqlc generate for SQL files in models/queries.

Generated code is written to models/internal/queries. This command is a
no-op when models/queries contains no .sql files with a -- name: annotation,
so unused sqlc support does not affect ordinary projects.`,
		Example: `  andurel generate queries`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			hasQueries, err := storage.HasSQLCQueryFiles(rootDir)
			if err != nil {
				return fmt.Errorf("check sqlc queries: %w", err)
			}
			commandsRun := []string{}
			warnings := []string{}
			if hasQueries {
				commandsRun = append(commandsRun, "sqlc generate", "go fmt ./models/internal/queries/...")
			} else {
				warnings = append(
					warnings,
					"no annotated sqlc query files found in models/queries; generation was skipped",
				)
			}

			return runMutation(cmd, mutationOptions{
				Action:      "generate queries",
				RootDir:     rootDir,
				CommandsRun: commandsRun,
				Warnings:    warnings,
				Breadcrumbs: []output.Breadcrumb{
					{Command: "andurel doctor --json", Description: "Check generated sqlc code for drift"},
				},
				Run: func(rootDir string) error {
					if !hasQueries {
						fmt.Println("No annotated sqlc query files found in models/queries; skipping generation.")
						return nil
					}
					return generateSQLCIfNeeded(rootDir)
				},
			})
		},
	}
	setAgentMetadata(
		cmd,
		"generation",
		"Runs only when models/queries contains a sqlc -- name: annotation. Returns a mutation report in structured modes.",
	)
	return cmd
}

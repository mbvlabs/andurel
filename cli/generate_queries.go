package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/spf13/cobra"
)

func newGenerateQueriesCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "queries",
		Aliases: []string{"q"},
		Short:   "Generate Go code from sqlc SQL queries",
		Long: `Run sqlc generate for SQL files in models/queries.

Generated code is written to models/internal/queries. This command is a
no-op when models/queries contains no .sql files, so unused sqlc support
does not affect ordinary projects.`,
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
			if !hasQueries {
				fmt.Println("No sqlc query files found in models/queries; skipping generation.")
				return nil
			}

			return generateSQLCIfNeeded(rootDir)
		},
	}
}

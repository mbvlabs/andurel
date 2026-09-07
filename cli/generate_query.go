package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/generator"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/internal/constants"
	"github.com/mbvlabs/andurel/internal/naming"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/spf13/cobra"
)

type sqlcQueryTemplateData struct {
	PascalName string
	TableName  string
}

func newGenerateQueryCommand() *cobra.Command {
	var (
		table  string
		dryRun bool
		diff   bool
	)

	cmd := &cobra.Command{
		Use:   "query NAME",
		Short: "Generate a new sqlc query file",
		Long: `Generates a new sqlc SQL query file in models/queries.

Pass the query group name in CamelCase, for example UserReport. The file is
created at models/queries/user_report.sql with sqlc annotation examples.

Use --table to scaffold an initial annotated query against an existing table.
After editing the SQL, run andurel generate queries to produce Go code in
models/internal/queries.`,
		Example: `  andurel generate query UserReport

      Creates models/queries/user_report.sql with commented examples.

  andurel generate query UserReport --table users

      Creates models/queries/user_report.sql with a starter ListUserReport query.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf(
					"too many arguments: query takes exactly 1 argument (the query name)",
				)
			}
			name := args[0]

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			return runMutation(cmd, mutationOptions{
				Action:   "generate query",
				Resource: name,
				RootDir:  rootDir,
				DryRun:   dryRun,
				Diff:     diff,
				Breadcrumbs: []output.Breadcrumb{
					{
						Command:     "andurel generate queries",
						Description: "Generate Go code from sqlc SQL files",
					},
				},
				Run: func(rootDir string) error {
					return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
						return generateSQLCQuery(name, table)
					})(cmd, args)
				},
			})
		},
	}

	cmd.Flags().StringVar(&table, "table", "", "Existing table name for a starter annotated query")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")
	setAgentMetadata(
		cmd,
		"generation",
		"Creates application-owned SQL in models/queries. Use sqlc for complex queries and keep generated models/internal/queries types behind the owning model package.",
	)

	return cmd
}

func generateSQLCQuery(name, table string) error {
	validator := generator.NewInputValidator()
	if err := validator.ValidateResourceName(name); err != nil {
		return output.WrapError(
			output.CodeUsage,
			fmt.Errorf("invalid query name: %w", err),
			output.ExitUsage,
			"Use a PascalCase query group name such as UserReport.",
		)
	}
	if table != "" {
		if err := validator.ValidateTableNameOverride(name, table); err != nil {
			return output.WrapError(
				output.CodeUsage,
				fmt.Errorf("invalid query table: %w", err),
				output.ExitUsage,
				"Use an existing snake_case SQL table identifier such as users.",
			)
		}
	}

	snakeName := naming.ToSnakeCase(name)
	pascalName := naming.ToPascalCase(snakeName)
	queryPath := filepath.Join(storage.SQLCQueriesDir, snakeName+".sql")

	if err := generateSQLCQueryFromTemplate(queryPath, sqlcQueryTemplateData{
		PascalName: pascalName,
		TableName:  table,
	}); err != nil {
		return fmt.Errorf("generate sqlc query file: %w", err)
	}

	fmt.Printf("Successfully generated sqlc query file %s\n", queryPath)
	return nil
}

func generateSQLCQueryFromTemplate(outputPath string, data sqlcQueryTemplateData) error {
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("file %s already exists", outputPath)
	}

	content, err := templates.RenderTemplateUsingGlobal("sqlc_query.tmpl", data)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(outputPath, []byte(content), constants.FilePermissionPrivate)
}

package cli

import (
	"github.com/mbvlabs/andurel/generator"
	"github.com/spf13/cobra"
)

func newQueriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query",
		Aliases: []string{"q"},
		Short:   "SQL query management",
		Long:    "Generate and compile SQL queries for database tables.",
	}

	cmd.AddCommand(
		newQueriesGenerateCommand(),
		newQueriesRefreshCommand(),
		newQueriesCompileCommand(),
	)

	return cmd
}

func newQueriesGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [table_name]",
		Short: "Generate CRUD queries for a database table",
		Long: `Generate SQL query file and SQLC types for a database table.
This is useful for tables that don't need a full model wrapper.

The command generates:
  - SQL queries file (database/queries/{table_name}.sql)
  - SQLC-generated query functions and types

The table name is used exactly as provided - no naming conventions are applied.
An error is returned if the table is not found in the migrations.

	Examples:
  andurel query generate user_roles           # Generate queries for 'user_roles' table
  andurel query generate users_organizations  # Generate queries for a junction table`,
		Args: cobra.ExactArgs(1),
		RunE: runQueriesGenerate,
	}

	return cmd
}

func runQueriesGenerate(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	tableName := args[0]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateQueriesOnly(tableName)
}

func newQueriesRefreshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh [table_name]",
		Short: "Refresh CRUD queries for a database table",
		Long: `Refresh an existing SQL query file and SQLC types for a database table.
This keeps the queries-only file in sync with the current table schema.

Examples:
  andurel query refresh user_roles          # Refresh queries for 'user_roles' table
  andurel query refresh users_organizations # Refresh queries for a junction table`,
		Args: cobra.ExactArgs(1),
		RunE: runQueriesRefresh,
	}
}

func runQueriesRefresh(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	tableName := args[0]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.RefreshQueriesOnly(tableName)
}

func newQueriesCompileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compile",
		Short: "Compile SQL queries and generate Go code",
		Long: `Compile SQL queries to check for errors and generate Go code.

This runs both 'sqlc compile' and 'sqlc generate' in sequence.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runSqlcCommand("compile"); err != nil {
				return err
			}
			return runSqlcCommand("generate")
		},
	}
}

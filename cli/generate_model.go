package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/generator"
	"github.com/spf13/cobra"
)

func newGenerateModelCommand() *cobra.Command {
	var (
		skipFactory     bool
		tableName       string
		updateModel     bool
		autoApply       bool
		primaryKeyColumn string
	)

	cmd := &cobra.Command{
		Use:   "model NAME",
		Short: "Generate a new model or update an existing one",
		Long: `Generates a new model or updates an existing one. Pass the model name in CamelCase.

When generating, the model is created from the existing database migration for the
table matching the model name. Fields, types, and timestamps are read
from the migration, so you don't have to specify them by hand.

For example, if a migration creates a "posts" table, running:

    andurel generate model Post

will generate a Post model with columns matching the posts table.

Use --update to sync an existing model file with migration changes.`,
		Example: `  andurel generate model Post

      Generates a Post model from the existing posts table migration.
      Model:   models/post.go
      Factory: models/factories/post.go

  andurel generate model User --table-name=people_data

      Generates a User model from the people_data table migration.

  andurel generate model Post --update

      Shows pending changes and prompts to apply them.

  andurel generate model Post --update --yes

      Applies model changes without prompting.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf("too many arguments: model takes exactly 1 argument (the model name)")
			}
			name := args[0]

			if err := chdirToProjectRoot(); err != nil {
				return err
			}

			if updateModel {
				return runModelUpdate(name, autoApply)
			}

			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				gen, err := generator.New()
				if err != nil {
					return err
				}
				if primaryKeyColumn != "" {
					return gen.GenerateModelWithPK(name, tableName, skipFactory, primaryKeyColumn)
				}
				return gen.GenerateModel(name, tableName, skipFactory)
			})(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&skipFactory, "skip-factory", false, "Skip generating a factory for the model")
	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name")
	cmd.Flags().BoolVar(&updateModel, "update", false, "Update an existing model from migration changes")
	cmd.Flags().BoolVar(&autoApply, "yes", false, "Apply changes without prompting for confirmation")
	cmd.Flags().StringVar(&primaryKeyColumn, "primary-key", "", "Specify the primary key column (skips interactive detection)")

	return cmd
}

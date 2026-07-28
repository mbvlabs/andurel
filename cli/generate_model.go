package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/generator"
	"github.com/spf13/cobra"
)

func newGenerateModelCommand() *cobra.Command {
	var (
		skipFactory      bool
		tableName        string
		updateModel      bool
		autoApply        bool
		primaryKeyColumn string
		dryRun           bool
		diff             bool
		modelMode        string
	)

	cmd := &cobra.Command{
		Use:     "model NAME",
		Aliases: []string{"m"},
		Short:   "Generate a new model or update an existing one",
		Long: `Generates a new model or updates an existing one. Pass the model name in CamelCase.

When generating, the model is created from the existing database migration for the
table matching the model name. Fields, types, and timestamps are read
from the migration, so you don't have to specify them by hand.

For example, if a migration creates a "posts" table, running:

    andurel generate model Post

will generate a Post model with columns matching the posts table.

Use --update to sync an existing model file with migration changes. Applying an
update also syncs the matching factory unless --skip-factory is passed.`,
		Example: `  andurel generate model Post

      Generates a Post model from the existing posts table migration.
      Model:   models/post.go
      Factory: models/factories/post.go

  andurel generate model User --table-name=people_data

      Generates a User model from the people_data table migration.

  andurel generate model Post --update

      Shows pending model and factory changes and prompts to apply them.

  andurel generate model Post --update --yes

      Applies model and factory changes without prompting.

  andurel generate model Post --update --skip-factory

      Applies model changes without syncing the factory.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf("too many arguments: model takes exactly 1 argument (the model name)")
			}
			name := args[0]
			mode := generator.ModelMode(modelMode)
			switch mode {
			case generator.ModelModeCRUD, generator.ModelModeReadOnly, generator.ModelModeCreateOnly:
			default:
				return fmt.Errorf("invalid model mode %q: expected crud, read-only, or create-only", modelMode)
			}

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			if dryRun && !updateModel {
				return runModelGenerationDryRun(cmd, rootDir, name, generator.ModelGenerationOptions{
					TableNameOverride: tableName,
					SkipFactory:       skipFactory,
					PrimaryKeyColumn:  primaryKeyColumn,
					Mode:              mode,
				}, diff)
			}

			return runMutation(cmd, mutationOptions{
				Action:   "generate model",
				Resource: name,
				RootDir:  rootDir,
				DryRun:   dryRun,
				Diff:     diff,
				Breadcrumbs: []output.Breadcrumb{
					{Command: "andurel doctor", Description: "Verify generated model health"},
				},
				Run: func(rootDir string) error {
					if updateModel {
						return runModelUpdateFunc(name, autoApply, skipFactory)
					}
					return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
						gen, err := newGenerator()
						if err != nil {
							return err
						}
						if mode != generator.ModelModeCRUD {
							return gen.GenerateModelWithMode(name, tableName, skipFactory, primaryKeyColumn, mode)
						}
						if primaryKeyColumn != "" {
							return gen.GenerateModelWithPK(name, tableName, skipFactory, primaryKeyColumn)
						}
						return gen.GenerateModel(name, tableName, skipFactory)
					})(cmd, args)
				},
			})
		},
	}

	cmd.Flags().BoolVar(&skipFactory, "skip-factory", false, "Skip generating or updating the matching factory")
	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name")
	cmd.Flags().BoolVar(&updateModel, "update", false, "Update an existing model from migration changes")
	cmd.Flags().BoolVar(&autoApply, "yes", false, "Apply changes without prompting for confirmation")
	cmd.Flags().StringVar(&primaryKeyColumn, "primary-key", "", "Specify the primary key column (skips interactive detection)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")
	cmd.Flags().StringVar(&modelMode, "mode", string(generator.ModelModeCRUD), "Generated operation mode: crud, read-only, or create-only")

	return cmd
}

func runModelGenerationDryRun(cmd *cobra.Command, rootDir, resourceName string, options generator.ModelGenerationOptions, includeDiff bool) error {
	outOpts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	gen, err := newGenerator()
	if err != nil {
		return err
	}
	plan, err := gen.PlanModel(resourceName, options)
	if err != nil {
		return err
	}
	report := buildModelPlanMutationReport(rootDir, resourceName, plan, includeDiff)
	if outOpts.Mode == output.ModeHuman && !outOpts.Quiet {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Dry run: %s\n", mutationSummary(report)); err != nil {
			return err
		}
		for _, path := range report.FilesCreated {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  create %s\n", path); err != nil {
				return err
			}
		}
		for _, path := range report.FilesUpdated {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  update %s\n", path); err != nil {
				return err
			}
		}
		return nil
	}
	return output.OK(cmd, report, mutationSummary(report), output.Breadcrumb{Command: "andurel doctor", Description: "Verify generated model health"})
}

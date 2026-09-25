package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/generator"
	"github.com/mbvlabs/andurel/v2/internal/naming"
	"github.com/spf13/cobra"
)

func newGenerateModelCommand() *cobra.Command {
	var (
		skipFactory      bool
		tableName        string
		primaryKeyColumn string
		dryRun           bool
		diff             bool
		modelMode        string
		customModel      bool
	)

	cmd := &cobra.Command{
		Use:     "model NAME",
		Aliases: []string{"m"},
		Short:   "Generate a new model",
		Long: `Generates a new model. Pass the model name in CamelCase.

The model is created from the existing database migration for the table
matching the model name. Fields, types, and timestamps are read from the
migration, so you don't have to specify them by hand.

For example, if a migration creates a "posts" table, running:

    andurel generate model Post

will generate a Post model with columns matching the posts table.

Use --custom to generate a non-table-backed model for narsilc custom queries.
Pass one or more field:type specs after the model name. No CRUD methods or
factory are generated; fill in models/queries/<name>.sql and run
andurel sync queries.

After a migration changes columns on an existing model, run
andurel sync model NAME.`,
		Example: `  andurel generate model Post

      Generates a Post model from the existing posts table migration.
      Model:   models/post.go
      Factory: models/factories/post.go

  andurel generate model User --table-name=people_data

      Generates a User model from the people_data table migration.

  andurel generate model AggregateResult --custom id:uuid name:string currency:int64

      Generates a custom (non-table) model with the given fields.
      Model: models/aggregate_result.go
      SQL:   models/queries/aggregate_result.sql

  andurel generate model Post --skip-factory

      Generates a Post model without a matching factory.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			name := args[0]
			fieldSpecs := args[1:]

			if customModel {
				if err := validateCustomModelFlags(cmd, tableName, primaryKeyColumn, fieldSpecs); err != nil {
					return err
				}
			} else if len(args) > 1 {
				return fmt.Errorf(
					"too many arguments: model takes exactly 1 argument (the model name); use --custom for field:type specs",
				)
			}

			mode := generator.ModelMode(modelMode)
			if !customModel {
				switch mode {
				case generator.ModelModeCRUD,
					generator.ModelModeReadOnly,
					generator.ModelModeCreateOnly:
				default:
					return fmt.Errorf(
						"invalid model mode %q: expected crud, read-only, or create-only",
						modelMode,
					)
				}
			}

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			if dryRun {
				if customModel {
					return runCustomModelGenerationDryRun(cmd, rootDir, name, fieldSpecs, diff)
				}
				return runModelGenerationDryRun(
					cmd,
					rootDir,
					name,
					generator.ModelGenerationOptions{
						TableNameOverride: tableName,
						SkipFactory:       skipFactory,
						PrimaryKeyColumn:  primaryKeyColumn,
						Mode:              mode,
					},
					diff,
				)
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
					return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
						gen, err := newGenerator()
						if err != nil {
							return err
						}
						if customModel {
							if err := gen.GenerateCustomModel(name, fieldSpecs); err != nil {
								return err
							}
							if err := generateNarsilcIfNeeded(rootDir); err != nil {
								return err
							}
							printGeneratedCustomModel(name)
							return nil
						}
						if mode != generator.ModelModeCRUD {
							if err := gen.GenerateModelWithMode(
								name,
								tableName,
								skipFactory,
								primaryKeyColumn,
								mode,
							); err != nil {
								return err
							}
						} else if primaryKeyColumn != "" {
							if err := gen.GenerateModelWithPK(
								name,
								tableName,
								skipFactory,
								primaryKeyColumn,
							); err != nil {
								return err
							}
						} else if err := gen.GenerateModel(name, tableName, skipFactory); err != nil {
							return err
						}
						if err := generateNarsilcIfNeeded(rootDir); err != nil {
							return err
						}
						printGeneratedModel(name, skipFactory)
						return nil
					})(cmd, args)
				},
			})
		},
	}

	cmd.Flags().
		BoolVar(&skipFactory, "skip-factory", false, "Skip generating the matching factory")
	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name")
	cmd.Flags().
		StringVar(&primaryKeyColumn, "primary-key", "", "Specify the primary key column (skips interactive detection)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")
	cmd.Flags().
		StringVar(&modelMode, "mode", string(generator.ModelModeCRUD), "Generated operation mode: crud, read-only, or create-only")
	cmd.Flags().
		BoolVar(&customModel, "custom", false, "Generate a non-table-backed model from field:type specs")

	return cmd
}

func validateCustomModelFlags(
	cmd *cobra.Command,
	tableName string,
	primaryKeyColumn string,
	fieldSpecs []string,
) error {
	var conflicts []string
	if tableName != "" || cmd.Flags().Changed("table-name") {
		conflicts = append(conflicts, "--table-name")
	}
	if primaryKeyColumn != "" || cmd.Flags().Changed("primary-key") {
		conflicts = append(conflicts, "--primary-key")
	}
	if cmd.Flags().Changed("mode") {
		conflicts = append(conflicts, "--mode")
	}
	if len(conflicts) > 0 {
		return fmt.Errorf(
			"--custom cannot be combined with %s",
			joinQuoted(conflicts),
		)
	}
	if len(fieldSpecs) == 0 {
		return fmt.Errorf(
			"--custom requires at least one field:type specification (e.g. id:uuid name:string)",
		)
	}
	return nil
}

func joinQuoted(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	out := ""
	for i, item := range items {
		if i == 0 {
			out = item
			continue
		}
		if i == len(items)-1 {
			out += " or " + item
			continue
		}
		out += ", " + item
	}
	return out
}

func runModelGenerationDryRun(
	cmd *cobra.Command,
	rootDir, resourceName string,
	options generator.ModelGenerationOptions,
	includeDiff bool,
) error {
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
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"Dry run: %s\n",
			mutationSummary(report),
		); err != nil {
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
	return output.OK(
		cmd,
		report,
		mutationSummary(report),
		output.Breadcrumb{Command: "andurel doctor", Description: "Verify generated model health"},
	)
}

func runCustomModelGenerationDryRun(
	cmd *cobra.Command,
	rootDir, resourceName string,
	fieldSpecs []string,
	includeDiff bool,
) error {
	outOpts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	gen, err := newGenerator()
	if err != nil {
		return err
	}
	plan, err := gen.PlanCustomModel(resourceName, fieldSpecs)
	if err != nil {
		return err
	}
	report := buildModelPlanMutationReport(rootDir, resourceName, plan, includeDiff)
	if outOpts.Mode == output.ModeHuman && !outOpts.Quiet {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"Dry run: %s\n",
			mutationSummary(report),
		); err != nil {
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
	return output.OK(
		cmd,
		report,
		mutationSummary(report),
		output.Breadcrumb{Command: "andurel doctor", Description: "Verify generated model health"},
	)
}

func printGeneratedModel(resourceName string, skipFactory bool) {
	snake := naming.ToSnakeCase(resourceName)
	fmt.Printf("✓ Generated SQL: models/queries/%s.sql\n", snake)
	fmt.Printf("✓ Generated typed queries: models/internal/queries\n")
	if !skipFactory {
		fmt.Printf("✓ Generated factory: models/factories/%s.go\n", snake)
	}
	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		resourceName,
	)
}

func printGeneratedCustomModel(resourceName string) {
	snake := naming.ToSnakeCase(resourceName)
	fmt.Printf("✓ Generated model: models/%s.go\n", snake)
	fmt.Printf("✓ Generated SQL stub: models/queries/%s.sql\n", snake)
	fmt.Printf(
		"Successfully generated custom model for %s (add narsilc queries, then run andurel sync queries)\n",
		resourceName,
	)
}

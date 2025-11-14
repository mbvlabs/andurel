package cli

import (
	"github.com/mbvlabs/andurel/generator"
	"github.com/mbvlabs/andurel/pkg/naming"

	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"g", "gen"},
		Short:   "Generate code and scaffolds",
		Long:    `Generate models, controllers, views, resources, and more.`,
	}

	generateCmd.AddCommand(newModelCommand())
	generateCmd.AddCommand(newControllerCommand())
	generateCmd.AddCommand(newViewCommand())
	generateCmd.AddCommand(newResourceCommand())

	return generateCmd
}

func newModelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model [name]",
		Short: "Generate a new model",
		Long: `Generate a new model with the specified name.
The model will include CRUD operations and database functions.
The table name is automatically inferred as the plural form of the model name.

Examples:
  andurel generate model User                        # Create new model for 'users' table
  andurel generate model User --table-name=accounts  # Create model using custom 'accounts' table
  andurel generate model User --refresh              # Refresh SQL queries and constructor functions`,
		Args: cobra.ExactArgs(1),
		RunE: generateModel,
	}

	cmd.Flags().
		Bool("refresh", false, "Refresh SQL queries and constructor functions - makes schema changes compiler-enforced")
	cmd.Flags().
		String("table-name", "", "Override the default table name (defaults to plural form of model name)")

	return cmd
}

func newViewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view [model_name]",
		Short: "Generate view templates for the specified model",
		Long: `Generate view templates for the specified resource.
The model must already exist before generating views.

By default, views are generated without controllers. Use --with-controller to also generate a resource controller.

Examples:
  andurel generate view User                    # Views without controller
  andurel generate view User --with-controller  # Views with controller`,
		Args: cobra.ExactArgs(1),
		RunE: generateView,
	}

	cmd.Flags().Bool("with-controller", false, "Generate controller along with the views")

	return cmd
}

func generateModel(cmd *cobra.Command, args []string) error {
	resourceName := args[0]

	refresh, err := cmd.Flags().GetBool("refresh")
	if err != nil {
		return err
	}

	tableNameOverride, err := cmd.Flags().GetString("table-name")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	if refresh {
		return gen.RefreshConstructors(
			resourceName,
			naming.DeriveTableName(resourceName),
		)
	}

	return gen.GenerateModel(resourceName, tableNameOverride)
}

func newControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller [model_name]",
		Short: "Generate a new resource controller with CRUD actions",
		Long: `Generate a new resource controller with full CRUD actions.
The controller will include index, show, new, create, edit, update, and destroy actions.
It will also generate the corresponding routes.

The model must already exist before generating a controller.

By default, controllers are generated without views. Use --with-views to also generate view templates.

Examples:
  andurel generate controller User              # Controller without views
  andurel generate controller User --with-views # Controller with views`,
		Args: cobra.ExactArgs(1),
		RunE: generateController,
	}

	cmd.Flags().Bool("with-views", false, "Generate views along with the controller")

	return cmd
}

func newResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource [name]",
		Short: "Generate a complete resource (model, controller, views, and routes)",
		Long: `Generate a complete resource including model, controller with CRUD actions, views, and routes.
This is equivalent to running model, controller, and view generators together.
The table name is automatically inferred as the plural form of the model name.

Examples:
  andurel generate resource Product    # Model + controller + views + routes for 'products' table`,
		Args: cobra.ExactArgs(1),
		RunE: generateResource,
	}

	return cmd
}

func generateController(cmd *cobra.Command, args []string) error {
	resourceName := args[0]

	withViews, err := cmd.Flags().GetBool("with-views")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateControllerFromModel(resourceName, withViews)
}

func generateResource(cmd *cobra.Command, args []string) error {
	resourceName := args[0]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	if err := gen.GenerateModel(resourceName, ""); err != nil {
		return err
	}

	return gen.GenerateControllerFromModel(resourceName, true)
}

func generateView(cmd *cobra.Command, args []string) error {
	resourceName := args[0]

	withController, err := cmd.Flags().GetBool("with-controller")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateViewFromModel(resourceName, withController)
}

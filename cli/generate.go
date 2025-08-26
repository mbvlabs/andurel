package cli

import (
	"mbvlabs/andurel/generator"

	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"g"},
		Short:   "Generate code and scaffolds",
		Long:    `Generate models, controllers, views, resources, and more.`,
	}

	generateCmd.AddCommand(newModelCommand())
	generateCmd.AddCommand(newControllerCommand())
	generateCmd.AddCommand(newResourceControllerCommand())
	generateCmd.AddCommand(newViewCommand())
	generateCmd.AddCommand(newResourceCommand())

	return generateCmd
}

func newModelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "model [name] [table]",
		Short: "Generate a new model",
		Long: `Generate a new model with the specified name.
The model will include CRUD operations and database functions.

Example:
  andurel generate model User`,
		Args: cobra.ExactArgs(2),
		RunE: generateModel,
	}
}

func newViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view [name]",
		Short: "Generate a new view",
		Long:  `Generate view templates for the specified resource.`,
		Args:  cobra.ExactArgs(1),
		RunE:  generateView,
	}
}

func generateModel(cmd *cobra.Command, args []string) error {
	resourceName := args[0]
	tableName := args[1]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateModel(resourceName, tableName)
}

func newControllerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "controller [name]",
		Short: "Generate a new basic controller",
		Long: `Generate a new basic controller without CRUD actions.
This creates a simple controller structure for custom actions.

Example:
  andurel generate controller Dashboard`,
		Args: cobra.ExactArgs(1),
		RunE: generateController,
	}
}

func newResourceControllerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resource_controller [name] [table]",
		Short: "Generate a new resource controller with CRUD actions",
		Long: `Generate a new resource controller with full CRUD actions.
The controller will include index, show, new, create, edit, update, and destroy actions.
It will also generate the corresponding routes.

Example:
  andurel generate resource_controller User users`,
		Args: cobra.ExactArgs(2),
		RunE: generateResourceController,
	}
}

func newResourceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resource [name] [table]",
		Short: "Generate a complete resource (model, resource controller, and routes)",
		Long: `Generate a complete resource including model, resource controller with CRUD actions, and routes.
This is equivalent to running model and resource_controller generators together.

Example:
  andurel generate resource Product products`,
		Args: cobra.ExactArgs(2),
		RunE: generateResource,
	}
}

func generateController(cmd *cobra.Command, args []string) error {
	resourceName := args[0]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateController(resourceName)
}

func generateResourceController(cmd *cobra.Command, args []string) error {
	resourceName := args[0]
	tableName := args[1]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateResourceController(resourceName, tableName)
}

func generateResource(cmd *cobra.Command, args []string) error {
	resourceName := args[0]
	tableName := args[1]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	// Generate model first
	if err := gen.GenerateModel(resourceName, tableName); err != nil {
		return err
	}

	// Then generate resource controller with CRUD operations
	return gen.GenerateResourceController(resourceName, tableName)
}

func generateView(cmd *cobra.Command, args []string) error {
	// resourceName := args[0]
	//
	// gen, err := generator.New()
	// if err != nil {
	// 	return err
	// }
	//
	// return gen.GenerateView(resourceName)
	//
	return nil
}

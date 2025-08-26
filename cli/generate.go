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

func newControllerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "controller [name]",
		Short: "Generate a new controller",
		Long: `Generate a new controller with CRUD actions for the specified resource.
The controller will include index, show, new, create, edit, update, and destroy actions.`,
		Args: cobra.ExactArgs(1),
		RunE: generateController,
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

func newResourceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resource [name]",
		Short: "Generate a complete resource (model, controller, views, and routes)",
		Long: `Generate a complete resource including model, controller, views, and routes.
This is equivalent to running model, controller, and view generators together.

Example:
  andurel generate resource Product`,
		Args: cobra.ExactArgs(1),
		RunE: generateResource,
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

func generateController(cmd *cobra.Command, args []string) error {
	// resourceName := args[0]
	//
	// gen, err := generator.New()
	// if err != nil {
	// return err
	// }

	// return gen.GenerateController(resourceName)

	return nil
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

func generateResource(cmd *cobra.Command, args []string) error {
	// resourceName := args[0]
	// tableName := args[1]
	//
	// gen, err := generator.New()
	// if err != nil {
	// 	return err
	// }
	//
	// if err := gen.GenerateModel(resourceName, tableName); err != nil {
	// 	return err
	// }
	//
	// return gen.GenerateController(resourceName)
	return nil
}

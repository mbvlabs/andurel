package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator"

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

func newViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view [name] [table]",
		Short: "Generate a new view",
		Long:  `Generate view templates for the specified resource.`,
		Args:  cobra.ExactArgs(2),
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
		Use:   "resource [name] [table]",
		Short: "Generate a complete resource (model, resource controller, and routes)",
		Long: `Generate a complete resource including model, resource controller with CRUD actions, and routes.
This is equivalent to running model and controller generators together.

By default, controllers are generated without views. Use --with-views to also generate view templates.

Examples:
  andurel generate resource Product products              # Model + controller without views
  andurel generate resource Product products --with-views # Model + controller with views`,
		Args: cobra.ExactArgs(2),
		RunE: generateResource,
	}
	
	cmd.Flags().Bool("with-views", false, "Generate views along with the controller")
	
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
	tableName := args[1]

	withViews, err := cmd.Flags().GetBool("with-views")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	if err := gen.GenerateModel(resourceName, tableName); err != nil {
		return err
	}

	return gen.GenerateController(resourceName, tableName, withViews)
}

func generateView(cmd *cobra.Command, args []string) error {
	resourceName := args[0]
	tableName := args[1]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	modelPath := filepath.Join("models", strings.ToLower(resourceName)+".go")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate the model first with: andurel generate model %s <table_name>",
			modelPath,
			resourceName,
		)
	}

	return gen.GenerateView(resourceName, tableName)
}

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGenerateScaffoldCommand() *cobra.Command {
	var (
		skipFactory      bool
		tableName        string
		primaryKeyColumn string
		vue              bool
	)

	cmd := &cobra.Command{
		Use:     "scaffold NAME",
		Aliases: []string{"s"},
		Short:   "Generate a complete scaffold resource",
		Long: `Scaffolds an entire resource, from model to controller and views, along
with routes. The resource is ready to use as a starting point for your
RESTful, resource-oriented application.

Pass the resource name in CamelCase as the first argument.

This is a convenience command that runs both:
  andurel generate model NAME
  andurel generate controller NAME

It generates the full set of CRUD actions: index, show, new, create,
edit, update, destroy.`,
		Example: `  andurel generate scaffold Post

      Generates a full Post resource with model, CRUD controller, views, and routes.
      Model:      models/post.go
      Factory:    models/factories/post.go
      Controller: controllers/posts.go
      Views:      views/posts_resource.templ
      Routes:     router/routes/posts.go
                  router/connect_posts_routes.go

  andurel generate scaffold User --table-name=people_data

      Generates a User resource from the people_data table.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf("too many arguments: scaffold takes exactly 1 argument (the resource name)")
			}
			name := args[0]

			if err := chdirToProjectRoot(); err != nil {
				return err
			}

			inertia := ""
			if vue {
				inertia = "vue"
			}

			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				gen, err := newGenerator()
				if err != nil {
					return err
				}

				return gen.GenerateScaffold(name, tableName, skipFactory, primaryKeyColumn, inertia)
			})(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&skipFactory, "skip-factory", false, "Skip generating a factory for the model")
	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name")
	cmd.Flags().StringVar(&primaryKeyColumn, "primary-key", "", "Specify the primary key column (skips interactive detection)")
	cmd.Flags().BoolVar(&vue, "vue", false, "Generate Inertia Vue views instead of Templ views")

	return cmd
}

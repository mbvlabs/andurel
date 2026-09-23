package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/internal/naming"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/spf13/cobra"
)

func newGenerateScaffoldCommand() *cobra.Command {
	var (
		skipFactory      bool
		tableName        string
		primaryKeyColumn string
		api              bool
		dryRun           bool
		diff             bool
	)

	cmd := &cobra.Command{
		Use:     "scaffold NAME",
		Aliases: []string{"s"},
		Short:   "Generate a complete scaffold resource",
		Long: `Scaffolds an entire resource, from model to controller and views, along
with routes. The resource is ready to use as a starting point for your
RESTful, resource-oriented application.

Pass the resource name in CamelCase as the first argument.
Names may include one lowercase namespace segment, such as admin/Widget.

This is a convenience command that runs both:
  andurel generate model NAME
  andurel generate controller NAME

It generates the full set of CRUD actions: index, show, new, create,
edit, update, destroy.

Use --api to generate a JSON API controller instead of views. The
scaffold creates the model and an API controller under controllers/api
with echo.JSON responses. No views are generated.

In Inertia projects, pages are generated for the adapter recorded in
andurel.lock. Templ/Datastar projects get templ views instead.`,
		Example: `  andurel generate scaffold Post

      Generates a full Post resource with model, CRUD controller, views, and routes.
      Model:      models/post.go
      Factory:    models/factories/post.go
      Controller: controllers/posts.go
      Views:      views/posts_resource.templ
      Routes:     router/routes/posts.go

  andurel generate scaffold admin/Widget

      Generates models/widget.go plus a namespaced controller, routes, and
      views under controllers/admin, router/*admin_widgets*, and
      views/admin_widgets_resource.templ.

  andurel generate scaffold User --api

      Generates a model, JSON API controller, and routes. No views.

  andurel generate scaffold User --table-name=people_data

      Generates a User resource from the people_data table.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf(
					"too many arguments: scaffold takes exactly 1 argument (the resource name)",
				)
			}
			name := args[0]
			namespace, resourceName, err := naming.ParseNamespacedResource(name)
			if err != nil {
				return err
			}
			if api {
				namespace = apiNamespace(namespace)
			}

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			inertiaAdapter := ""
			if !api {
				inertiaAdapter, err = projectUIAdapter(rootDir)
				if err != nil {
					return err
				}
			}

			return runMutation(cmd, mutationOptions{
				Action:   "generate scaffold",
				Resource: name,
				RootDir:  rootDir,
				DryRun:   dryRun,
				Diff:     diff,
				Breadcrumbs: []output.Breadcrumb{
					{
						Command:     "andurel db migrate up",
						Description: "Apply migrations before using the resource",
					},
					{Command: "andurel run", Description: "Start the development server"},
				},
				Run: func(rootDir string) error {
					return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
						gen, err := newGenerator()
						if err != nil {
							return err
						}

						if err := gen.GenerateScaffold(
							resourceName,
							namespace,
							tableName,
							skipFactory,
							primaryKeyColumn,
							inertiaAdapter,
							api,
						); err != nil {
							return err
						}
						if err := generateNarsilcIfNeeded(rootDir); err != nil {
							return err
						}
						return refreshInertiaGeneratedTypeScript(rootDir, inertiaAdapter, api)
					})(cmd, args)
				},
			})
		},
	}

	cmd.Flags().
		BoolVar(&skipFactory, "skip-factory", false, "Skip generating a factory for the model")
	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name")
	cmd.Flags().
		StringVar(&primaryKeyColumn, "primary-key", "", "Specify the primary key column (skips interactive detection)")
	cmd.Flags().BoolVar(&api, "api", false, "Generate a JSON API controller under controllers/api")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")
	setAgentMetadata(
		cmd,
		"generation",
		"Follows project UI from andurel.lock (Inertia pages or Templ). --api is mutually exclusive with UI page generation.",
	)

	return cmd
}

func refreshInertiaGeneratedTypeScript(rootDir, inertia string, isAPI bool) error {
	if err := refreshRoutesTSAfterInertiaGeneration(rootDir, inertia, isAPI); err != nil {
		return err
	}
	return refreshPayloadsTSAfterInertiaGeneration(rootDir, inertia, isAPI)
}

func refreshRoutesTSAfterInertiaGeneration(rootDir, inertia string, isAPI bool) error {
	if isAPI || !layout.IsSupportedInertiaAdapter(inertia) {
		return nil
	}
	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		return err
	}
	_, err = generateRoutesJSFile(rootDir, manifest)
	return err
}

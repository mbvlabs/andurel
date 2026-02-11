package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/generator"

	"github.com/spf13/cobra"
)

// chdirToProjectRoot finds the go.mod root and changes the working directory
// to it so that the generator's relative paths resolve correctly regardless
// of where the command was invoked.
func chdirToProjectRoot() error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return fmt.Errorf("not in an andurel project directory: %w", err)
	}
	return os.Chdir(rootDir)
}

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
	generateCmd.AddCommand(newFragmentCommand())

	return generateCmd
}

func newModelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "model [name]",
		Aliases: []string{"m"},
		Short:   "Generate a new model",
		Long: `Generate a new model with the specified name.
The model will include CRUD operations and database functions.
The table name is automatically inferred as the plural form of the model name.

Examples:
  andurel generate model User                        # Create new model for 'users' table
  andurel generate model User --table-name=accounts  # Create model using custom 'accounts' table
  andurel generate model User --skip-factory         # Skip factory generation`,
		Args: cobra.ExactArgs(1),
		RunE: withGenerateCleanup(generateModel),
	}

	cmd.Flags().
		String("table-name", "", "Override the default table name (defaults to plural form of model name)")
	cmd.Flags().
		Bool("skip-factory", false, "Skip factory generation")

	return cmd
}

func newViewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "view [model_name]",
		Aliases: []string{"v"},
		Short:   "Generate view templates for the specified model",
		Long: `Generate view templates for the specified resource.
The model must already exist before generating views.

By default, views are generated without controllers. Use --with-controller to also generate a resource controller.

Examples:
  andurel generate view User                    # Views without controller
  andurel generate view User --with-controller  # Views with controller`,
		Args: cobra.ExactArgs(1),
		RunE: withGenerateCleanup(generateView),
	}

	cmd.Flags().Bool("with-controller", false, "Generate controller along with the views")

	return cmd
}

func generateModel(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	resourceName := args[0]

	tableNameOverride, err := cmd.Flags().GetString("table-name")
	if err != nil {
		return err
	}

	skipFactory, err := cmd.Flags().GetBool("skip-factory")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateModel(resourceName, tableNameOverride, skipFactory)
}

func newControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "controller [model_name]",
		Aliases: []string{"c"},
		Short:   "Generate a new resource controller with CRUD actions",
		Long: `Generate a new resource controller with full CRUD actions.
The controller will include index, show, new, create, edit, update, and destroy actions.
It will also generate the corresponding routes.

The model must already exist before generating a controller.

By default, controllers are generated without views. Use --with-views to also generate view templates.

Examples:
  andurel generate controller User              # Controller without views
  andurel generate controller User --with-views # Controller with views`,
		Args: cobra.ExactArgs(1),
		RunE: withGenerateCleanup(generateController),
	}

	cmd.Flags().Bool("with-views", false, "Generate views along with the controller")

	return cmd
}

func newResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resource [name]",
		Aliases: []string{"r"},
		Short:   "Generate a complete resource (model, controller, views, and routes)",
		Long: `Generate a complete resource including model, controller with CRUD actions, views, and routes.
This is equivalent to running model, controller, and view generators together.
The table name is automatically inferred as the plural form of the model name.

Examples:
  andurel generate resource Product                        # Model + controller + views + routes for 'products' table
  andurel generate resource Feedback --table-name=user_feedback  # Use custom table name`,
		Args: cobra.ExactArgs(1),
		RunE: withGenerateCleanup(generateResource),
	}

	cmd.Flags().
		String("table-name", "", "Override the default table name (defaults to plural form of model name)")

	return cmd
}

func generateController(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

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
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	resourceName := args[0]

	tableNameOverride, err := cmd.Flags().GetString("table-name")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	// Generate resource always generates factory by default
	if err := gen.GenerateModel(resourceName, tableNameOverride, false); err != nil {
		return err
	}

	return gen.GenerateControllerFromModel(resourceName, true)
}

func generateView(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

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

func newFragmentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fragment [controller_name] [method_name] [path]",
		Short: "Add a method, route, and registration to an existing controller",
		Long: `Add a new method stub, route variable, and route registration to an existing controller.
The controller, routes, and connect files must already exist.

The route type is auto-detected from path parameters:
  :id    -> NewRouteWithUUIDID (or NewRouteWithSerialID/NewRouteWithBigSerialID/NewRouteWithStringID based on existing routes)
  :slug  -> NewRouteWithSlug
  :token -> NewRouteWithToken
  :file  -> NewRouteWithFile
  none   -> NewSimpleRoute

Examples:
  andurel generate fragment Webhook Validate /validate
  andurel generate fragment Article ShowBySlug /:slug --method GET
  andurel generate fragment Order Approve /:id/approve --method POST`,
		Args: cobra.ExactArgs(3),
		RunE: withGenerateCleanup(generateFragment),
	}

	cmd.Flags().String("method", "GET", "HTTP method (GET, POST, PUT, DELETE, PATCH)")

	return cmd
}

func generateFragment(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	controllerName := args[0]
	methodName := args[1]
	path := args[2]

	httpMethod, err := cmd.Flags().GetString("method")
	if err != nil {
		return err
	}

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateFragment(generator.FragmentConfig{
		ControllerName: controllerName,
		MethodName:     methodName,
		Path:           path,
		HTTPMethod:     strings.ToUpper(httpMethod),
	})
}

type createdFileTracker struct {
	rootDir       string
	existingFiles map[string]struct{}
}

func withGenerateCleanup(run func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		tracker, trackerErr := newCreatedFileTracker()

		runErr := run(cmd, args)
		if runErr == nil {
			return nil
		}

		if trackerErr != nil {
			return formatGenerateFailure(runErr, nil, nil, trackerErr)
		}

		removedFiles, cleanupFailures, cleanupErr := tracker.cleanupCreatedFiles()
		if cleanupErr != nil {
			return formatGenerateFailure(runErr, nil, nil, cleanupErr)
		}

		return formatGenerateFailure(runErr, removedFiles, cleanupFailures, nil)
	}
}

func newCreatedFileTracker() (*createdFileTracker, error) {
	rootDir, err := findProjectRoot()
	if err != nil {
		return nil, err
	}

	existingFiles, err := snapshotFiles(rootDir)
	if err != nil {
		return nil, err
	}

	return &createdFileTracker{
		rootDir:       rootDir,
		existingFiles: existingFiles,
	}, nil
}

func (t *createdFileTracker) cleanupCreatedFiles() ([]string, []string, error) {
	currentFiles, err := snapshotFiles(t.rootDir)
	if err != nil {
		return nil, nil, err
	}

	createdFiles := make([]string, 0)
	for relPath := range currentFiles {
		if _, exists := t.existingFiles[relPath]; !exists {
			createdFiles = append(createdFiles, relPath)
		}
	}
	sort.Strings(createdFiles)

	removedFiles := make([]string, 0, len(createdFiles))
	cleanupFailures := make([]string, 0)

	for _, relPath := range createdFiles {
		fullPath := filepath.Join(t.rootDir, relPath)
		if err := os.Remove(fullPath); err != nil {
			cleanupFailures = append(cleanupFailures, fmt.Sprintf("%s (%v)", relPath, err))
			continue
		}
		removedFiles = append(removedFiles, relPath)
	}

	return removedFiles, cleanupFailures, nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, statErr := os.Stat(goModPath); statErr == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("unable to locate project root containing go.mod")
		}
		dir = parent
	}
}

func snapshotFiles(rootDir string) (map[string]struct{}, error) {
	files := make(map[string]struct{})

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		files[filepath.ToSlash(relPath)] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func formatGenerateFailure(runErr error, removedFiles []string, cleanupFailures []string, trackerErr error) error {
	if trackerErr != nil {
		return fmt.Errorf("%w\n\nUnable to clean up created files automatically: %v", runErr, trackerErr)
	}

	var msg strings.Builder
	msg.WriteString(runErr.Error())

	if len(removedFiles) == 0 && len(cleanupFailures) == 0 {
		msg.WriteString("\n\nNo new files were created before the failure.")
		return errors.New(msg.String())
	}

	msg.WriteString("\n\nGeneration failed and automatic cleanup ran.")
	if len(removedFiles) > 0 {
		msg.WriteString(fmt.Sprintf("\nRemoved %d created file(s):", len(removedFiles)))
		msg.WriteString(formatPathList(removedFiles, 12))
	}

	if len(cleanupFailures) > 0 {
		msg.WriteString(fmt.Sprintf("\nCould not remove %d file(s):", len(cleanupFailures)))
		msg.WriteString(formatPathList(cleanupFailures, 12))
		msg.WriteString("\nPlease remove these files manually.")
	}

	return errors.New(msg.String())
}

func formatPathList(paths []string, maxItems int) string {
	var out strings.Builder
	limit := len(paths)
	if limit > maxItems {
		limit = maxItems
	}

	for i := 0; i < limit; i++ {
		out.WriteString("\n  - ")
		out.WriteString(paths[i])
	}

	if len(paths) > limit {
		out.WriteString(fmt.Sprintf("\n  - ... and %d more", len(paths)-limit))
	}

	return out.String()
}

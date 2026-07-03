package cli

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/mbvlabs/andurel/layout"

	"github.com/spf13/cobra"
)

func newProjectCommand(version string) *cobra.Command {
	projectCmd := &cobra.Command{
		Use:     "new [project-name]",
		Aliases: []string{"n"},
		Short:   "Create a new Andurel project",
		Long: `Scaffold a complete Andurel project with the given name.

Generates the full project structure including controllers, models, views,
database migrations, router, services, and configuration files. After
creation, run 'andurel tool sync' to download required binaries.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if isInAndurelProject() {
				return fmt.Errorf("cannot create a new project inside an existing Andurel project")
			}
			return newProject(cmd, args, version)
		},
	}

	projectCmd.Flags().
		StringSliceP("extensions", "e", nil, "Extensions to enable (comma-separated list)")

	projectCmd.Flags().
		String("inertia", "", "Inertia adapter to use (vue, react). Optionally append /npm|pnpm|bun|yarn to specify the JS runtime (default: npm)")

	return projectCmd
}

func newProject(cmd *cobra.Command, args []string, version string) error {
	projectName := args[0]
	basePath := "./" + projectName

	// If the target directory is ".", use the current directory
	if args[0] == "." {
		// Get the current directory
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		// Get the current directory contents
		files, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		// If the current directory is empty, use the current directory as the project name
		if len(files) != 0 {
			return fmt.Errorf("current directory is not empty")
		}
		projectName = path.Base(dir)
		basePath = "./"
	}

	database := "postgresql"
	cssFramework := "tailwind"

	inertiaFlag, err := cmd.Flags().GetString("inertia")
	if err != nil {
		return err
	}

	adapter := inertiaFlag
	javascriptRuntime := ""
	if inertiaFlag != "" {
		parts := strings.SplitN(inertiaFlag, "/", 2)
		adapter = parts[0]
		if len(parts) == 2 {
			javascriptRuntime = parts[1]
		} else {
			javascriptRuntime = "npm"
		}

		if !layout.IsSupportedInertiaAdapter(adapter) {
			return fmt.Errorf(
				"invalid inertia adapter: %s - valid options are 'vue', 'react'",
				adapter,
			)
		}
		if !layout.IsSupportedJavaScriptRuntime(javascriptRuntime) {
			return fmt.Errorf(
				"invalid JavaScript runtime: %s - valid options are 'npm', 'pnpm', 'bun', 'yarn'",
				javascriptRuntime,
			)
		}
	}

	extensions, err := cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		return err
	}
	if err := layout.Scaffold(basePath, projectName, database, cssFramework, version, extensions, "uberfx", adapter, javascriptRuntime); err != nil {
		return err
	}

	fmt.Printf("\n🎉 Successfully created project: %s\n", projectName)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", args[0])
	fmt.Printf("  andurel tool sync\n")
	fmt.Printf("  cp .env.example .env\n")
	fmt.Printf("  fill in your database connection details in .env\n")
	fmt.Printf("  (andurel database create - if database does not exist\n")
	fmt.Printf("  andurel database migrate up\n")
	if layout.IsSupportedInertiaAdapter(adapter) {
		fmt.Printf("  %s install\n", javascriptRuntime)
	}
	fmt.Printf("  andurel run\n")

	return nil
}

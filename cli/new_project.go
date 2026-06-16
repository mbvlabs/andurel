package cli

import (
	"fmt"
	"os"
	"path"

	"github.com/mbvlabs/andurel/layout"

	"github.com/spf13/cobra"
)

func newProjectCommand(version string) *cobra.Command {
	projectCmd := &cobra.Command{
		Use:     "new [project-name]",
		Aliases: []string{"n"},
		Short: "Create a new Andurel project",
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
		StringP("css", "c", "", "CSS framework to use (tailwind, vanilla) (optional, default: tailwind)")

	projectCmd.Flags().
		String("frontend", "", "Frontend view layer (templ, inertia-vue) (optional, default: templ)")

	projectCmd.Flags().
		StringSliceP("extensions", "e", nil, "Extensions to enable (comma-separated list)")

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

	cssFramework, err := cmd.Flags().GetString("css")
	if err != nil {
		return err
	}

	if cssFramework == "" {
		cssFramework = "tailwind"
	}

	if cssFramework != "tailwind" && cssFramework != "vanilla" {
		return fmt.Errorf(
			"invalid css framework provided: %s - valid options are 'tailwind' and 'vanilla'",
			cssFramework,
		)
	}

	viewLayer, err := cmd.Flags().GetString("frontend")
	if err != nil {
		return err
	}

	if viewLayer == "" {
		viewLayer = "templ"
	}

	if viewLayer != "templ" && viewLayer != "inertia-vue" {
		return fmt.Errorf(
			"invalid frontend view layer provided: %s - valid options are 'templ' and 'inertia-vue'",
			viewLayer,
		)
	}

	extensions, err := cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		return err
	}
	if err := layout.Scaffold(basePath, projectName, database, cssFramework, viewLayer, version, extensions); err != nil {
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
	fmt.Printf("  andurel run\n")

	return nil
}

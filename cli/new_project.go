package cli

import (
	"fmt"
	"github.com/mbvlabs/andurel/layout"

	"github.com/spf13/cobra"
)

func newProjectCommand() *cobra.Command {
	projectCmd := &cobra.Command{
		Use:   "new [project-name]",
		Short: "Create a new Andurel project",
		Long: `Create a new Andurel project with the specified name.

This will scaffold a complete project structure with all necessary files,
dependencies, and configuration.`,
		Args: cobra.ExactArgs(1),
		RunE: newProject,
	}

	projectCmd.Flags().
		StringP("repo", "r", "", "GitHub username (i.e. mbvlabs or github.com/mbvlabs (optional)")

	return projectCmd
}

func newProject(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	basePath := "./" + projectName

	moduleNamerepo, err := cmd.Flags().GetString("repo")
	if err != nil {
		return err
	}

	if moduleNamerepo != "" {
		projectName = moduleNamerepo + "/" + projectName
	}

	if err := layout.Scaffold(basePath, projectName); err != nil {
		return err
	}

	fmt.Printf("\n🎉 Successfully created project: %s\n", projectName)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  npm i\n")
	fmt.Printf("  update .env\n")
	fmt.Printf("  and just run\n")

	return nil
}

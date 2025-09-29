package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/layout"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
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

	projectCmd.Flags().
		StringP("database", "d", "", "Database to use (postgresql, sqlite) (optional, default: postgres)")

	projectCmd.Flags().
		StringSliceP("recipes", "R", []string{}, "Recipes to include (comma-separated: auth, etc.)")

	return projectCmd
}

func newProject(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	basePath := "./" + projectName

	repo, err := cmd.Flags().GetString("repo")
	if err != nil {
		return err
	}

	database, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}

	if database == "" {
		database = "postgresql"
	}

	if database != "" && database != "sqlite" && database != "postgresql" {
		return fmt.Errorf(
			"invalid database provided: %s - valid options are 'postgresql' and 'sqlite'",
			database,
		)
	}

	recipes, err := cmd.Flags().GetStringSlice("recipes")
	if err != nil {
		return err
	}

	validRecipes := []string{"auth"}
	for _, recipe := range recipes {
		if !slices.Contains(validRecipes, recipe) {
			return fmt.Errorf("invalid recipe: %s - valid options are: %s", recipe, strings.Join(validRecipes, ", "))
		}
	}

	if err := layout.Scaffold(basePath, projectName, repo, database, recipes); err != nil {
		return err
	}

	fmt.Printf("\n🎉 Successfully created project: %s\n", projectName)
	if slices.Contains(recipes, "auth") {
		fmt.Printf("  Auth recipe enabled - visit /login or /signup\n")
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", args[0])
	if database == "postgresql" {
		fmt.Printf("  cp .env.example .env and fill it out\n")
	}
	fmt.Printf("  and andurel a run\n")

	return nil
}

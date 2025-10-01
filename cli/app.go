package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func newAppCommand() *cobra.Command {
	appCmd := &cobra.Command{
		Use:     "app",
		Aliases: []string{"a"},
		Short:   "App related commands",
		Long:    "Commands related to running and managing the application.",
	}

	appCmd.AddCommand(newTailwindCommand())
	appCmd.AddCommand(newConsoleCommand())

	return appCmd
}

func newTailwindCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tailwind",
		Short: "Sets up Tailwind CSS for the project",
		Long:  "Sets up Tailwind CSS for the project. If no system is specified, it defaults to 'npm'",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return layout.SetupTailwind(".")
		},
	}

	return cmd
}

func newConsoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "console",
		Aliases: []string{"c"},
		Short:   "Runs an interactive 'console' to interact with the database.",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			binPath := filepath.Join(wd, "bin", "console")
			if _, err := os.Stat(binPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"console binary not found at %s; build it with 'go build ./cmd/console'",
						binPath,
					)
				}
				return err
			}

			command := exec.Command(binPath, args...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin

			return command.Run()
		},
	}

	return cmd
}

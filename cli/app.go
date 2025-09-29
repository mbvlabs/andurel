package cli

import (
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

	appCmd.AddCommand(newRunAppCommand())
	appCmd.AddCommand(newTailwindCommand())

	return appCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs the app",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			binPath := filepath.Join(wd, "bin", "run")

			runCmd := exec.Command(binPath)
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			runCmd.Stdin = os.Stdin

			return runCmd.Run()
		},
	}

	return cmd
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

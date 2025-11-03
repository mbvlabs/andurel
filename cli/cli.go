// Package cli provides the command-line interface for the Andurel framework.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewRootCommand(version, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "andurel",
		Short:   "Andurel - The Go Web development framework",
		Long:    `Andurel is a comprehensive web development framework for Go,`,
		Version: fmt.Sprintf("%s (built: %s)", version, date),
	}

	rootCmd.AddCommand(newRunAppCommand())

	rootCmd.AddCommand(newProjectCommand())
	rootCmd.AddCommand(newGenerateCommand())
	rootCmd.AddCommand(newMigrationCommand())
	rootCmd.AddCommand(newSqlcCommand())

	rootCmd.AddCommand(newAppCommand())

	return rootCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs the app",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "run")

			runCmd := exec.Command(binPath)
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			runCmd.Stdin = os.Stdin

			return runCmd.Run()
		},
	}

	return cmd
}

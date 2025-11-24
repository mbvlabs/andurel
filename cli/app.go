package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newAppCommand() *cobra.Command {
	appCmd := &cobra.Command{
		Use:     "app",
		Aliases: []string{"a"},
		Short:   "App related commands",
		Long:    "Commands related to running and managing the application.",
	}

	appCmd.AddCommand(newConsoleCommand())
	appCmd.AddCommand(newMailpitCommand())

	return appCmd
}

func newConsoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "console",
		Aliases: []string{"c"},
		Short:   "Runs an interactive database console",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "console")
			if _, err := os.Stat(binPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"console binary not found at %s\nRun 'andurel sync' to build it",
						binPath,
					)
				}
				return err
			}

			command := exec.Command(binPath, args...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin
			command.Dir = rootDir

			return command.Run()
		},
	}

	return cmd
}

func newMailpitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mailpit",
		Aliases: []string{"m"},
		Short:   "Runs the Mailpit email server",
		Long: `Runs the Mailpit email server with default configuration.

Default bindings:
  - SMTP: 0.0.0.0:1025
  - HTTP: 0.0.0.0:8025

Override defaults by passing flags, e.g.:
  andurel app mailpit --smtp=0.0.0.0:2525`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "mailpit")
			if _, err := os.Stat(binPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"mailpit binary not found at %s\nRun 'andurel sync' to download it",
						binPath,
					)
				}
				return err
			}

			defaultArgs := []string{
				"--smtp=0.0.0.0:1025",
				"--listen=0.0.0.0:8025",
			}

			finalArgs := append(defaultArgs, args...)

			command := exec.Command(binPath, finalArgs...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin
			command.Dir = rootDir

			return command.Run()
		},
	}

	return cmd
}

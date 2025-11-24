package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout/cmds"
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
	appCmd.AddCommand(newMailhogCommand())
	appCmd.AddCommand(newMailhogSetupCommand())

	return appCmd
}

func newTailwindCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tailwind",
		Short: "Sets up Tailwind CSS for the project",
		Long:  "Sets up Tailwind CSS for the project. If no system is specified, it defaults to 'npm'",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmds.SetupTailwind(".")
		},
	}

	return cmd
}

func newConsoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "console",
		Aliases: []string{"c"},
		Short:   "Runs an interactive database console",
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

			return command.Run()
		},
	}

	return cmd
}

func newMailhogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mailhog",
		Aliases: []string{"m"},
		Short:   "Runs the MailHog email server",
		Long: `Runs the MailHog email server with default configuration.

Default bindings:
  - SMTP: 0.0.0.0:1025
  - HTTP: 0.0.0.0:8025

Override defaults by passing flags, e.g.:
  andurel app mailhog --smtp-bind-addr=0.0.0.0:2525`,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			binPath := filepath.Join(wd, "bin", "mailhog")
			if _, err := os.Stat(binPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"mailhog binary not found at %s; run 'andurel app mailhog-setup' to download it",
						binPath,
					)
				}
				return err
			}

			defaultArgs := []string{
				"--smtp-bind-addr=0.0.0.0:1025",
				"--api-bind-addr=0.0.0.0:8025",
			}

			finalArgs := append(defaultArgs, args...)

			command := exec.Command(binPath, finalArgs...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin

			return command.Run()
		},
	}

	return cmd
}

func newMailhogSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mailhog-setup",
		Short: "Downloads and sets up MailHog binary",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmds.SetupMailHog(".")
		},
	}

	return cmd
}

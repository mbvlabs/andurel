package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newMigrationCommand() *cobra.Command {
	migrationCmd := &cobra.Command{
		Use:     "migration",
		Aliases: []string{"m", "mig"},
		Short:   "Database migration helpers",
		Long:    "Manage database migrations for the current project using the generated migration binary.",
	}

	migrationCmd.AddCommand(
		newMigrationNewCommand(),
		newMigrationUpCommand(),
		newMigrationDownCommand(),
		newMigrationFixCommand(),
		newMigrationResetCommand(),
		newMigrationUpToCommand(),
		newMigrationDownToCommand(),
	)

	return migrationCmd
}

func newMigrationNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new SQL migration",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary(append([]string{"new"}, args...)...)
		},
		Example: "andurel migration new create_users_table",
	}
}

func newMigrationUpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary("up")
		},
	}
}

func newMigrationDownCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Rollback the most recent migration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary("down")
		},
	}
}

func newMigrationFixCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fix",
		Short: "Re-number migrations to fix gaps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary("fix")
		},
	}
}

func newMigrationResetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset database by rolling back all migrations and reapplying them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary("reset")
		},
	}
}

func newMigrationUpToCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up-to [version]",
		Short: "Apply migrations up to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary("up-to", args[0])
		},
	}
}

func newMigrationDownToCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down-to [version]",
		Short: "Rollback migrations down to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrationBinary("down-to", args[0])
		},
	}
}

func runMigrationBinary(args ...string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	binPath := filepath.Join(rootDir, "bin", "migration")
	if _, err := os.Stat(binPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"migration binary not found at %s\nRun 'andurel sync' to build it",
				binPath,
			)
		}
		return err
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}

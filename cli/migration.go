package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
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

	godotenv.Load()

	dbKind := os.Getenv("DB_KIND")
	dbPort := os.Getenv("DB_PORT")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbSllMode := os.Getenv("DB_SSL_MODE")

	if dbKind == "" || dbPort == "" || dbHost == "" || dbName == "" || dbUser == "" ||
		dbPass == "" ||
		dbSllMode == "" {
		return fmt.Errorf("database configuration environment variables are not fully set")
	}

	databaseURL := fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		dbKind,
		dbUser,
		dbPass,
		dbHost,
		dbPort,
		dbName,
		dbSllMode,
	)
	driver, dbString := parseDatabaseURL(databaseURL)

	goosePath := filepath.Join(rootDir, "bin", "goose")
	if _, err := os.Stat(goosePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"goose binary not found at %s\nRun 'andurel tool sync' to download it",
				goosePath,
			)
		}
		return err
	}

	migrationDir := filepath.Join(rootDir, "database", "migrations")

	gooseArgs := append([]string{"-dir", migrationDir, driver, dbString}, args...)

	cmd := exec.Command(goosePath, gooseArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}

func parseDatabaseURL(url string) (driver, dbString string) {
	if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
		return "postgres", url
	}
	if after, ok := strings.CutPrefix(url, "sqlite://"); ok {
		return "sqlite3", after
	}
	if after, ok := strings.CutPrefix(url, "sqlite3://"); ok {
		return "sqlite3", after
	}
	return "postgres", url
}

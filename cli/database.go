package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/spf13/cobra"
)

func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Database management commands",
		Long:    "Commands for managing database seeds.",
	}

	cmd.AddCommand(
		newDBSeedCommand(),
	)

	return cmd
}

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate",
		Aliases: []string{"migration"},
		Short:   "Database migration helpers",
		Long:    "Manage database migrations for the current project using goose.",
	}

	cmd.AddCommand(
		newDBMigrationNewCommand(),
		newDBMigrationUpCommand(),
		newDBMigrationDownCommand(),
		newDBMigrationStatusCommand(),
		newDBMigrationFixCommand(),
		newDBMigrationResetCommand(),
		newDBMigrationUpToCommand(),
		newDBMigrationDownToCommand(),
	)

	return cmd
}

func newQueriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "queries",
		Aliases: []string{"query", "q"},
		Short:   "SQL query code generation (sqlc)",
		Long:    "Manage SQLC code generation for the current project.",
	}

	cmd.AddCommand(
		newDBQueriesCompileCommand(),
		newDBQueriesGenerateCommand(),
	)

	return cmd
}

func newSeedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Run database seeds",
		Long: `Run the database seed file at database/seeds/main.go.

Edit this file to add your seed data using model factories.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeed()
		},
	}
}

// Migration commands

func newDBMigrationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migration",
		Aliases: []string{"m", "mig"},
		Short:   "Database migration helpers",
		Long:    "Manage database migrations for the current project using goose.",
	}

	cmd.AddCommand(
		newDBMigrationNewCommand(),
		newDBMigrationUpCommand(),
		newDBMigrationDownCommand(),
		newDBMigrationStatusCommand(),
		newDBMigrationFixCommand(),
		newDBMigrationResetCommand(),
		newDBMigrationUpToCommand(),
		newDBMigrationDownToCommand(),
	)

	return cmd
}

func newDBMigrationNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "new [name]",
		Short:   "Create a new SQL migration",
		Args:    cobra.MinimumNArgs(1),
		Example: "andurel db migration new create_users_table",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := []string{"create"}
			c = append(c, args...)
			c = append(c, "sql")

			return runGoose(c...)
		},
	}
}

func newDBMigrationUpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("up")
		},
	}
}

func newDBMigrationDownCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Rollback the most recent migration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("down")
		},
	}
}

func newDBMigrationFixCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fix",
		Short: "Re-number migrations to fix gaps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("fix")
		},
	}
}

func newDBMigrationResetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset database by rolling back all migrations and reapplying them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("reset")
		},
	}
}

func newDBMigrationUpToCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up-to [version]",
		Short: "Apply migrations up to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("up-to", args[0])
		},
	}
}

func newDBMigrationDownToCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down-to [version]",
		Short: "Rollback migrations down to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("down-to", args[0])
		},
	}
}

// Queries commands (sqlc)

func newDBQueriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "queries",
		Aliases: []string{"q"},
		Short:   "SQL query code generation (sqlc)",
		Long:    "Manage SQLC code generation for the current project.",
	}

	cmd.AddCommand(
		newDBQueriesCompileCommand(),
		newDBQueriesGenerateCommand(),
	)

	return cmd
}

func newDBQueriesCompileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compile",
		Short: "Compile SQL queries to check for errors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSqlcCommand("compile")
		},
	}
}

func newDBQueriesGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from SQL queries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSqlcCommand("generate")
		},
	}
}

func newDBMigrationStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current migration status",
		Long:  "Display the current migration version and list all migrations with their status.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("status")
		},
	}
}

// Seed command

func newDBSeedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Run database seeds",
		Long: `Run the database seed file at database/seeds/main.go.

Edit this file to add your seed data using model factories.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeed()
		},
	}
}

func runSeed() error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	seedDir := filepath.Join(rootDir, "database", "seeds")
	mainFile := filepath.Join(seedDir, "main.go")

	if _, err := os.Stat(mainFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"seed file not found at %s\nRun 'andurel generate seed' to create it",
				mainFile,
			)
		}
		return err
	}

	cmd := exec.Command("go", "run", "./database/seeds")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}

// Shared helpers

func runGoose(args ...string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	godotenv.Load()

	driver, dbString, err := buildDatabaseURL()
	if err != nil {
		return err
	}

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

func runSqlcCommand(action string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	configPath := filepath.Join(rootDir, "database", "sqlc.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"sqlc config not found at %s",
				configPath,
			)
		}
		return err
	}

	var cmd *exec.Cmd

	if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
		cmd = exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@"+versions.Sqlc, "-f", configPath, action)
	} else {
		sqlcBin := filepath.Join(rootDir, "bin", "sqlc")
		if _, err := os.Stat(sqlcBin); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf(
					"sqlc binary not found at %s\nRun 'andurel tool sync' to download it",
					sqlcBin,
				)
			}
			return err
		}
		cmd = exec.Command(sqlcBin, "-f", configPath, action)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}

func buildDatabaseURL() (driver, dbString string, err error) {
	dbKind := os.Getenv("DB_KIND")
	dbPort := os.Getenv("DB_PORT")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbSslMode := os.Getenv("DB_SSL_MODE")

	var missing []string
	if dbKind == "" {
		missing = append(missing, "DB_KIND")
	}
	if dbPort == "" {
		missing = append(missing, "DB_PORT")
	}
	if dbHost == "" {
		missing = append(missing, "DB_HOST")
	}
	if dbName == "" {
		missing = append(missing, "DB_NAME")
	}
	if dbUser == "" {
		missing = append(missing, "DB_USER")
	}
	if dbPass == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if dbSslMode == "" {
		missing = append(missing, "DB_SSL_MODE")
	}

	if len(missing) > 0 {
		return "", "", fmt.Errorf("missing database configuration environment variables: %v", missing)
	}

	databaseURL := fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		dbKind,
		dbUser,
		dbPass,
		dbHost,
		dbPort,
		dbName,
		dbSslMode,
	)

	return "postgres", databaseURL, nil
}

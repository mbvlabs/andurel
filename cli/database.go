package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "database",
		Aliases: []string{"d", "db"},
		Short:   "Database management commands",
		Long:    "Commands for managing database seeds.",
	}

	cmd.AddCommand(
		newDBSeedCommand(),
		newDBDropCommand(),
		newDBCreateCommand(),
		newDBNukeCommand(),
		newDBRebuildCommand(),
	)

	return cmd
}

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate",
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

// Lifecycle commands

func newDBDropCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop the configured database",
		Long:  "Drop the configured database using the connection details from .env.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return dropDatabase(force)
		},
	}

	cmd.Flags().
		BoolVar(&force, "force", false, "Allow dropping system databases like postgres/template1")

	return cmd
}

func newDBCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create the configured database",
		Long:  "Create the configured database using the connection details from .env.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createDatabase()
		},
	}
}

func newDBNukeCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "nuke",
		Short: "Drop and recreate the configured database",
		Long:  "Drop and recreate the configured database using the connection details from .env.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nukeDatabase(force)
		},
	}

	cmd.Flags().
		BoolVar(&force, "force", false, "Allow dropping system databases like postgres/template1")

	return cmd
}

func newDBRebuildCommand() *cobra.Command {
	var force bool
	var skipSeed bool

	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Drop, recreate, migrate, and seed the database",
		Long:  "Drop, recreate, migrate, and seed the database using the connection details from .env.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return rebuildDatabase(force, skipSeed)
		},
	}

	cmd.Flags().
		BoolVar(&force, "force", false, "Allow dropping system databases like postgres/template1")
	cmd.Flags().
		BoolVar(&skipSeed, "skip-seed", false, "Skip running database seeds after migrations")

	return cmd
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

type dbConfig struct {
	Kind     string
	Port     string
	Host     string
	Name     string
	User     string
	Password string
	SslMode  string
}

func loadDatabaseConfig() (dbConfig, error) {
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
		return dbConfig{}, fmt.Errorf(
			"missing database configuration environment variables: %v",
			missing,
		)
	}

	return dbConfig{
		Kind:     dbKind,
		Port:     dbPort,
		Host:     dbHost,
		Name:     dbName,
		User:     dbUser,
		Password: dbPass,
		SslMode:  dbSslMode,
	}, nil
}

func dropDatabase(force bool) error {
	godotenv.Load()

	_, dbURL, err := buildDatabaseURL()
	if err != nil {
		return err
	}

	confirmed, err := confirmDestructive("drop", dbURL)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(os.Stdout, "Aborted.")
		return nil
	}

	cfg, conn, ctx, cancel, err := openAdminConnection()
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close(ctx)

	return dropDatabaseWithConn(ctx, cfg, conn, force)
}

func createDatabase() error {
	godotenv.Load()

	cfg, conn, ctx, cancel, err := openAdminConnection()
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close(ctx)

	return createDatabaseWithConn(ctx, cfg, conn)
}

func nukeDatabase(force bool) error {
	godotenv.Load()

	_, dbURL, err := buildDatabaseURL()
	if err != nil {
		return err
	}

	confirmed, err := confirmDestructive("nuke", dbURL)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(os.Stdout, "Aborted.")
		return nil
	}

	cfg, conn, ctx, cancel, err := openAdminConnection()
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close(ctx)

	if err := dropDatabaseWithConn(ctx, cfg, conn, force); err != nil {
		return err
	}

	return createDatabaseWithConn(ctx, cfg, conn)
}

func rebuildDatabase(force bool, skipSeed bool) error {
	if err := nukeDatabase(force); err != nil {
		return err
	}

	if err := runGoose("up"); err != nil {
		return err
	}

	if skipSeed {
		return nil
	}

	return runSeed()
}

func openAdminConnection() (dbConfig, *pgx.Conn, context.Context, context.CancelFunc, error) {
	cfg, err := loadDatabaseConfig()
	if err != nil {
		return dbConfig{}, nil, nil, nil, err
	}

	if strings.ToLower(cfg.Kind) != "postgres" {
		return dbConfig{}, nil, nil, nil, fmt.Errorf(
			"database lifecycle commands only support postgres",
		)
	}

	adminDB := "postgres"
	if cfg.Name == adminDB {
		adminDB = "template1"
	}

	adminURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		adminDB,
		cfg.SslMode,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		cancel()
		return dbConfig{}, nil, nil, nil, err
	}

	return cfg, conn, ctx, cancel, nil
}

func dropDatabaseWithConn(ctx context.Context, cfg dbConfig, conn *pgx.Conn, force bool) error {
	if isSystemDatabase(cfg.Name) && !force {
		return fmt.Errorf("refusing to drop system database %q without --force", cfg.Name)
	}

	if _, err := conn.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", cfg.Name); err != nil {
		return err
	}

	_, err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdentifier(cfg.Name)))
	return err
}

func createDatabaseWithConn(ctx context.Context, cfg dbConfig, conn *pgx.Conn) error {
	if isSystemDatabase(cfg.Name) {
		return fmt.Errorf("refusing to create system database %q", cfg.Name)
	}

	_, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(cfg.Name)))
	return err
}

func isSystemDatabase(name string) bool {
	switch strings.ToLower(name) {
	case "postgres", "template0", "template1":
		return true
	default:
		return false
	}
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func confirmDestructive(action string, dbURL string) (bool, error) {
	if strings.TrimSpace(dbURL) == "" {
		return false, errors.New("database URL is empty")
	}

	fmt.Fprintf(os.Stdout, "Are you sure you want to %s this database: %s y/N ", action, dbURL)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
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

	cmd := exec.Command(sqlcBin, "-f", configPath, action)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}

func buildDatabaseURL() (driver, dbString string, err error) {
	cfg, err := loadDatabaseConfig()
	if err != nil {
		return "", "", err
	}

	databaseURL := fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Kind,
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.SslMode,
	)

	return "postgres", databaseURL, nil
}

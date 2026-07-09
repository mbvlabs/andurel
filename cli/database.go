package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/cli/output"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

type seedReport struct {
	Name   string   `json:"name,omitempty"`
	Names  []string `json:"names,omitempty"`
	Output []string `json:"output,omitempty"`
}

type commandOutputRunner func(rootDir string, args []string, stdin io.Reader, stdout, stderr io.Writer) ([]byte, error)

var runSeedCommandOutput commandOutputRunner = func(rootDir string, args []string, stdin io.Reader, stdout, stderr io.Writer) ([]byte, error) {
	runCmd := exec.Command("go", args...)
	runCmd.Stdin = stdin
	runCmd.Dir = rootDir
	if stdout != nil || stderr != nil {
		runCmd.Stdout = stdout
		runCmd.Stderr = stderr
		return nil, runCmd.Run()
	}
	return runCmd.CombinedOutput()
}

var runGooseCommand = func(rootDir, goosePath string, args []string) error {
	cmd := exec.Command(goosePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir
	return cmd.Run()
}

type adminConnection interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Close(ctx context.Context) error
}

var openAdminConnectionFunc = openAdminConnection
var runGooseFunc = runGoose
var runSeedFunc = runSeed

var errDatabaseOperationAborted = errors.New("database operation aborted")

func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "database",
		Aliases: []string{"d", "db"},
		Short:   "Database management commands",
		Long: `Commands for managing your Andurel project's database lifecycle:
create, drop, nuke, rebuild, seed, and run migrations.

Use the subcommands below to manage your database.`,
	}
	setAgentMetadata(cmd, "database", "Database lifecycle commands. Prefer --json or --agent for automation; destructive commands may prompt unless --force is provided.")

	cmd.AddCommand(
		newDBSeedCommand(),
		newDBDropCommand(),
		newDBCreateCommand(),
		newDBNukeCommand(),
		newDBRebuildCommand(),
		newMigrateCommand(),
	)

	return cmd
}

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate",
		Aliases: []string{"m", "mig"},
		Short:   "Database migration helpers",
		Long: `Manage database migrations for the current project using goose.

Migrations live in database/migrations/ as SQL files. Create a new
migration, apply pending ones, rollback, check status, or fix gaps.`,
		Example: `  andurel database migrate new add_user_role
  andurel database migrate up
  andurel database migrate status
  andurel database migrate down
  andurel database migrate reset`,
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

// Migration commands

func newDBMigrationNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "new [name]",
		Aliases: []string{"n"},
		Short:   "Create a new SQL migration",
		Long: `Create a new SQL migration file in database/migrations/.
The name should describe the change, e.g. "create_users_table".`,
		Args:    cobra.MinimumNArgs(1),
		Example: "  andurel database migrate new create_users_table",
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
		Long:  "Apply any pending migrations that have not yet been run against the database.",
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
		Long:  "Roll back the most recently applied migration.",
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
		Long:  "Re-number sequential migrations to close any gaps in the numbering.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("fix")
		},
	}
}

func newDBMigrationResetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "reset",
		Aliases: []string{"rs"},
		Short:   "Roll back all migrations and re-apply them",
		Long:    "Roll back every migration (down), then re-apply them all (up).",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("reset")
		},
	}
}

func newDBMigrationUpToCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "up-to [version]",
		Aliases: []string{"upto"},
		Short:   "Apply migrations up to a specific version",
		Long:    "Apply migrations only up to (and including) the given version number.",
		Args:    cobra.ExactArgs(1),
		Example: "  andurel database migrate up-to 20250101120000",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("up-to", args[0])
		},
	}
}

func newDBMigrationDownToCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "down-to [version]",
		Aliases: []string{"downto"},
		Short:   "Rollback migrations down to a specific version",
		Long:    "Roll back migrations down to (but not including) the given version number.",
		Args:    cobra.ExactArgs(1),
		Example: "  andurel database migrate down-to 20250101120000",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("down-to", args[0])
		},
	}
}

func newDBMigrationStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Show current migration status",
		Long:    "Display the current migration version and list all migrations with their status.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoose("status")
		},
	}
}

// Seed command

func newDBSeedCommand() *cobra.Command {
	var list bool

	cmd := &cobra.Command{
		Use:     "seed [name]",
		Aliases: []string{"s"},
		Short:   "Run database seeds",
		Long: `Run the database seed entrypoint at cmd/seeds.

Edit database/seeds to add reusable named seed sets using model factories.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  andurel database seed
  andurel database seed development
  andurel database seed test
  andurel database seed --list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runSeed(cmd, name, list)
		},
	}
	setAgentMetadata(cmd, "database", "Runs the v1 seed entrypoint at cmd/seeds. Use --list to discover available named seeds.")

	cmd.Flags().BoolVar(&list, "list", false, "List available seeds")

	return cmd
}

// Lifecycle commands

func newDBDropCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop the configured database",
		Long: `Drop the configured database using the connection details from .env.

Uses --force to override protection on system databases (e.g.,
postgres, template1). This cannot be undone.`,
		Args:    cobra.NoArgs,
		Example: "  andurel database drop\n  andurel database drop --force",
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
		Use:     "create",
		Aliases: []string{"crt"},
		Short:   "Create the configured database",
		Long: `Create the configured database using the connection details from .env.

Reads DB_HOST, DB_PORT, DB_NAME, DB_USER, and DB_PASSWORD to connect
and create the database. No-op if the database already exists.`,
		Args:    cobra.NoArgs,
		Example: "  andurel database create",
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
		Long: `Drop and recreate the configured database using the connection details
from .env.

This is a destructive operation that drops the database and creates a
fresh empty one. Use --force to override system database protection.`,
		Args:    cobra.NoArgs,
		Example: "  andurel database nuke\n  andurel database nuke --force",
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
	var seedName string

	cmd := &cobra.Command{
		Use:     "rebuild",
		Aliases: []string{"rb"},
		Short:   "Drop, recreate, migrate, and seed the database",
		Long: `Drop, recreate, migrate, and seed the database.

This is a full database reset:
  1. Drops the existing database
  2. Creates a fresh one
  3. Runs all pending migrations
  4. Seeds the database through cmd/seeds

Use --seed to choose a named seed set. Use --skip-seed to skip step 4.
Use --force to override system
database protection for the drop step.`,
		Args:    cobra.NoArgs,
		Example: "  andurel database rebuild\n  andurel database rebuild --seed development\n  andurel database rebuild --skip-seed",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rebuildDatabase(cmd, force, skipSeed, seedName)
		},
	}
	setAgentMetadata(cmd, "database", "Drops, recreates, migrates, then runs cmd/seeds. Use --seed to select the seed set.")

	cmd.Flags().
		BoolVar(&force, "force", false, "Allow dropping system databases like postgres/template1")
	cmd.Flags().
		BoolVar(&skipSeed, "skip-seed", false, "Skip running database seeds after migrations")
	cmd.Flags().
		StringVar(&seedName, "seed", "", "Seed name to run after migrations")

	return cmd
}

func runSeed(cmd *cobra.Command, name string, list bool) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	cmdSeedsMain := filepath.Join(rootDir, "cmd", "seeds", "main.go")
	if _, err := os.Stat(cmdSeedsMain); err != nil {
		if os.IsNotExist(err) {
			return output.NewError(
				output.CodeMissingTool,
				fmt.Sprintf("seed entrypoint not found at %s", cmdSeedsMain),
				output.ExitDependency,
				fmt.Sprintf("Create %s or run andurel new to scaffold a v1 project.", cmdSeedsMain),
			)
		}
		return err
	}

	goArgs := []string{"run", "./cmd/seeds"}
	if list {
		goArgs = append(goArgs, "--list")
	}
	if name != "" {
		goArgs = append(goArgs, name)
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if !output.UsesStructuredOutput(opts) {
		_, err := runSeedCommandOutput(rootDir, goArgs, os.Stdin, os.Stdout, os.Stderr)
		return err
	}

	out, err := runSeedCommandOutput(rootDir, goArgs, os.Stdin, nil, nil)
	lines := splitNonEmptyLines(string(out))
	if err != nil {
		return output.WrapError(
			output.CodeExternalCommandFailed,
			fmt.Errorf("run seed command: %w", err),
			output.ExitExternal,
			strings.Join(lines, "\n"),
		)
	}

	if list {
		return output.OK(cmd, seedReport{Names: lines}, fmt.Sprintf("Found %d seed sets", len(lines)))
	}

	seedName := name
	if seedName == "" {
		seedName = "default"
	}
	return output.OK(
		cmd,
		seedReport{Name: seedName, Output: lines},
		fmt.Sprintf("Ran %q seed", seedName),
		output.Breadcrumb{Command: "andurel database seed --list", Description: "List available seed sets"},
	)
}

func splitNonEmptyLines(value string) []string {
	raw := strings.Split(value, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// Shared helpers

func runGoose(args ...string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	loadProjectEnv(rootDir)

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

	return runGooseCommand(rootDir, goosePath, gooseArgs)
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
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	loadProjectEnv(rootDir)

	cfg, err := loadDatabaseConfig()
	if err != nil {
		return err
	}

	confirmed, err := confirmDestructive("drop", cfg.Name)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(os.Stdout, "Aborted.")
		return errDatabaseOperationAborted
	}

	cfg, conn, ctx, cancel, err := openAdminConnectionFunc()
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close(ctx)

	if err := dropDatabaseWithConn(ctx, cfg, conn, force); err != nil {
		return err
	}

	fmt.Printf("Database %q dropped successfully.\n", cfg.Name)
	return nil
}

func createDatabase() error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	loadProjectEnv(rootDir)

	cfg, conn, ctx, cancel, err := openAdminConnectionFunc()
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close(ctx)

	if err := createDatabaseWithConn(ctx, cfg, conn); err != nil {
		return err
	}

	fmt.Printf("Database %q created successfully.\n", cfg.Name)
	return nil
}

func nukeDatabase(force bool) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	loadProjectEnv(rootDir)

	cfg, err := loadDatabaseConfig()
	if err != nil {
		return err
	}

	confirmed, err := confirmDestructive("nuke", cfg.Name)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(os.Stdout, "Aborted.")
		return errDatabaseOperationAborted
	}

	cfg, conn, ctx, cancel, err := openAdminConnectionFunc()
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close(ctx)

	if err := dropDatabaseWithConn(ctx, cfg, conn, force); err != nil {
		return err
	}

	if err := createDatabaseWithConn(ctx, cfg, conn); err != nil {
		return err
	}

	fmt.Printf("Database %q nuked successfully.\n", cfg.Name)
	return nil
}

func rebuildDatabase(cmd *cobra.Command, force bool, skipSeed bool, seedName string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	loadProjectEnv(rootDir)

	cfg, err := loadDatabaseConfig()
	if err != nil {
		return err
	}

	if err := nukeDatabase(force); err != nil {
		return err
	}

	if err := runGooseFunc("up"); err != nil {
		return err
	}

	if skipSeed {
		return nil
	}

	if err := runSeedFunc(cmd, seedName, false); err != nil {
		return err
	}

	fmt.Printf("Database %q rebuilt successfully.\n", cfg.Name)
	return nil
}

func openAdminConnection() (dbConfig, adminConnection, context.Context, context.CancelFunc, error) {
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

	adminURL := databaseURL(cfg, adminDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		cancel()
		return dbConfig{}, nil, nil, nil, fmt.Errorf(
			"connect to postgres admin database %q on %s failed",
			adminDB,
			net.JoinHostPort(cfg.Host, cfg.Port),
		)
	}

	return cfg, conn, ctx, cancel, nil
}

func dropDatabaseWithConn(ctx context.Context, cfg dbConfig, conn adminConnection, force bool) error {
	if isSystemDatabase(cfg.Name) && !force {
		return fmt.Errorf("refusing to drop system database %q without --force", cfg.Name)
	}

	if _, err := conn.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", cfg.Name); err != nil {
		return err
	}

	_, err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdentifier(cfg.Name)))
	return err
}

func createDatabaseWithConn(ctx context.Context, cfg dbConfig, conn adminConnection) error {
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

func confirmDestructive(action string, databaseName string) (bool, error) {
	if strings.TrimSpace(databaseName) == "" {
		return false, errors.New("database name is empty")
	}

	fmt.Fprintf(os.Stdout, "Are you sure you want to %s database %q? y/N ", action, databaseName)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func loadProjectEnv(rootDir string) {
	envPath := filepath.Join(rootDir, ".env")
	_ = godotenv.Load(envPath)
}

func buildDatabaseURL() (driver, dbString string, err error) {
	cfg, err := loadDatabaseConfig()
	if err != nil {
		return "", "", err
	}

	rawURL := databaseURL(cfg, cfg.Name)

	return "postgres", rawURL, nil
}

func databaseURL(cfg dbConfig, databaseName string) string {
	query := url.Values{}
	query.Set("sslmode", cfg.SslMode)
	escapedPath := "/" + url.PathEscape(databaseName)

	return (&url.URL{
		Scheme:   cfg.Kind,
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     "/" + databaseName,
		RawPath:  escapedPath,
		RawQuery: query.Encode(),
	}).String()
}

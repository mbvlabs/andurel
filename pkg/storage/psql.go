// Package storage provides abstractions for database interactions and default implementations.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// Executor is the query interface satisfied by *bun.DB, *bun.Tx, and *bun.Conn.
// It remains available for application-owned model and factory APIs.
type Executor = bun.IDB

// Connection exposes the Bun executor and its underlying database/sql pool.
type Connection interface {
	Executor() bun.IDB
	DB() *sql.DB
	BeginTransaction(ctx context.Context, opts *sql.TxOptions) (Transaction, error)
}

// Postgres wraps a bun.DB.
type Postgres struct {
	bun *bun.DB
}

var _ Connection = (*Postgres)(nil)

// NewPostgres creates a new database connection from the supplied configuration.
func NewPostgres(ctx context.Context, config Config, options ...Option) (*Postgres, error) {
	settings := postgresOptions{
		config:            config,
		runtimeParameters: make(map[string]string),
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&settings); err != nil {
			return nil, fmt.Errorf("storage: configure postgres: %w", err)
		}
	}

	if !settings.telemetrySet && settings.config.OpenTelemetry {
		settings.telemetry = &TelemetryConfig{}
	}
	if settings.connectTimeout != nil {
		settings.config.ConnectTimeout = *settings.connectTimeout
	}
	if settings.statementCacheCapacity != nil {
		settings.config.StatementCacheCapacity = *settings.statementCacheCapacity
	}
	if settings.descriptionCacheCapacity != nil {
		settings.config.DescriptionCacheCapacity = *settings.descriptionCacheCapacity
	}
	if settings.maxOpenConnections != nil {
		settings.config.MaxOpenConnections = *settings.maxOpenConnections
	}
	if settings.maxIdleConnections != nil {
		settings.config.MaxIdleConnections = *settings.maxIdleConnections
	}
	if settings.connectionMaxLifetime != nil {
		settings.config.ConnectionMaxLifetime = *settings.connectionMaxLifetime
	}
	if settings.connectionMaxIdleTime != nil {
		settings.config.ConnectionMaxIdleTime = *settings.connectionMaxIdleTime
	}

	databaseURL := settings.configURL
	if databaseURL == "" {
		var err error
		databaseURL, err = settings.config.DatabaseURL()
		if err != nil {
			return nil, fmt.Errorf("storage: configure postgres: %w", err)
		}
	}
	pgxConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("storage: parse database URL: %w", err)
	}
	if settings.telemetry != nil {
		pgxConfig.Tracer = otelpgx.NewTracer(telemetryOptions(*settings.telemetry)...)
	}
	// URL settings retain pgx's parsed defaults and URL query parameters.
	// Explicit field options take precedence for either configuration source.
	if settings.configURL == "" {
		pgxConfig.ConnectTimeout = settings.config.ConnectTimeout
		pgxConfig.StatementCacheCapacity = settings.config.StatementCacheCapacity
		pgxConfig.DescriptionCacheCapacity = settings.config.DescriptionCacheCapacity
		if settings.config.ApplicationName != "" {
			pgxConfig.RuntimeParams["application_name"] = settings.config.ApplicationName
		}
	}
	if settings.connectTimeout != nil {
		pgxConfig.ConnectTimeout = *settings.connectTimeout
	}
	if settings.applicationName != nil {
		pgxConfig.RuntimeParams["application_name"] = *settings.applicationName
	}
	if settings.statementCacheCapacity != nil {
		pgxConfig.StatementCacheCapacity = *settings.statementCacheCapacity
	}
	if settings.descriptionCacheCapacity != nil {
		pgxConfig.DescriptionCacheCapacity = *settings.descriptionCacheCapacity
	}
	if pgxConfig.StatementCacheCapacity == 0 && pgxConfig.DefaultQueryExecMode == pgx.QueryExecModeCacheStatement {
		pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	if pgxConfig.DescriptionCacheCapacity == 0 && pgxConfig.DefaultQueryExecMode == pgx.QueryExecModeCacheDescribe {
		pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	}
	maps.Copy(pgxConfig.RuntimeParams, settings.runtimeParameters)
	if settings.tlsConfig != nil {
		pgxConfig.TLSConfig = settings.tlsConfig.Clone()
		for i := range pgxConfig.Fallbacks {
			pgxConfig.Fallbacks[i].TLSConfig = settings.tlsConfig.Clone()
		}
	}

	sqldb := stdlib.OpenDB(*pgxConfig)
	maxOpenConnections := settings.config.MaxOpenConnections
	if settings.maxOpenConnections != nil {
		maxOpenConnections = *settings.maxOpenConnections
	}
	sqldb.SetMaxOpenConns(maxOpenConnections)
	maxIdleConnections := settings.config.MaxIdleConnections
	if settings.maxIdleConnections != nil {
		maxIdleConnections = *settings.maxIdleConnections
	}
	sqldb.SetMaxIdleConns(maxIdleConnections)
	connectionMaxLifetime := settings.config.ConnectionMaxLifetime
	if settings.connectionMaxLifetime != nil {
		connectionMaxLifetime = *settings.connectionMaxLifetime
	}
	sqldb.SetConnMaxLifetime(connectionMaxLifetime)
	connectionMaxIdleTime := settings.config.ConnectionMaxIdleTime
	if settings.connectionMaxIdleTime != nil {
		connectionMaxIdleTime = *settings.connectionMaxIdleTime
	}
	sqldb.SetConnMaxIdleTime(connectionMaxIdleTime)
	db := bun.NewDB(sqldb, pgdialect.New())

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: ping database: %w", err)
	}

	return &Postgres{bun: db}, nil
}

// Executor returns the Bun query executor.
func (p *Postgres) Executor() bun.IDB {
	return p.bun
}

// DB returns the underlying sql.DB.
func (p *Postgres) DB() *sql.DB {
	return p.bun.DB
}

// Close closes the database connection.
func (p *Postgres) Close() error {
	return p.bun.Close()
}

// BeginTransaction starts a transaction with Bun and database/sql access.
func (p *Postgres) BeginTransaction(
	ctx context.Context,
	opts *sql.TxOptions,
) (Transaction, error) {
	tx, err := p.bun.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("storage: begin transaction: %w", err)
	}

	return bunTransaction{tx: tx}, nil
}

// TestCluster owns a Postgres test container and can create isolated databases.
type TestCluster struct {
	container *postgres.PostgresContainer
	host      string
	port      string
	user      string
	password  string
	adminDB   string
}

// NewTestCluster starts a Postgres container for a test package.
func NewTestCluster(ctx context.Context) (*TestCluster, error) {
	const (
		user     = "testuser"
		password = "testpass"
		adminDB  = "postgres"
	)

	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(adminDB),
		postgres.WithUsername(user),
		postgres.WithPassword(password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: start postgres container: %w", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, fmt.Errorf("storage: get postgres port: %w", err)
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, fmt.Errorf("storage: get postgres host: %w", err)
	}

	return &TestCluster{
		container: pgContainer,
		host:      host,
		port:      port.Port(),
		user:      user,
		password:  password,
		adminDB:   adminDB,
	}, nil
}

// Close terminates the Postgres test container.
func (tc *TestCluster) Close(ctx context.Context) error {
	if tc == nil || tc.container == nil {
		return nil
	}
	return tc.container.Terminate(ctx)
}

// NewTestDB creates a migrated, isolated database for one test.
func (tc *TestCluster) NewTestDB(t testing.TB, migrations fs.FS, migrationDir string) Connection {
	t.Helper()

	if migrations == nil {
		t.Fatal("migrations filesystem is required")
	}
	if strings.TrimSpace(migrationDir) == "" {
		t.Fatal("migration directory is required")
	}

	ctx := context.Background()
	name := fmt.Sprintf("test_%d", time.Now().UnixNano())

	admin, err := NewPostgres(ctx, DefaultConfig(), WithDatabaseURL(tc.databaseURL(tc.adminDB)))
	if err != nil {
		t.Fatalf("failed to connect to admin database: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Close()
	})

	if _, err := admin.DB().ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	dropped := false
	t.Cleanup(func() {
		if dropped {
			return
		}
		_, _ = admin.DB().
			ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})

	db, err := NewPostgres(ctx, DefaultConfig(), WithDatabaseURL(tc.databaseURL(name)))
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.DB().
			ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		dropped = true
	})

	if err := RunMigrations(ctx, db.DB(), migrations, migrationDir); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func (tc *TestCluster) databaseURL(name string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		tc.user,
		tc.password,
		tc.host,
		tc.port,
		name,
	)
}

// RunMigrations applies Goose migrations from an embedded filesystem.
func RunMigrations(ctx context.Context, db *sql.DB, migrations fs.FS, migrationDir string) error {
	entries, err := fs.ReadDir(migrations, migrationDir)
	if err != nil {
		return fmt.Errorf("storage: read migrations: %w", err)
	}

	hasSQL := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			hasSQL = true
			break
		}
	}
	if !hasSQL {
		return fmt.Errorf("storage: no SQL migrations found in %s", migrationDir)
	}

	migrationFS, err := fs.Sub(migrations, migrationDir)
	if err != nil {
		return fmt.Errorf("storage: open migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS)
	if err != nil {
		return fmt.Errorf("storage: create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("storage: apply migrations: %w", err)
	}

	return nil
}

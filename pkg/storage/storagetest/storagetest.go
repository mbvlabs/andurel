// Package storagetest starts Postgres test containers and creates isolated,
// migrated databases for tests. It lives outside package storage so production
// applications do not link testcontainers or its Docker client dependencies;
// package storage itself imports neither testing nor testcontainers.
package storagetest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Cluster owns a Postgres test container and can create isolated databases.
type Cluster struct {
	container *postgres.PostgresContainer
	host      string
	port      string
	user      string
	password  string
	adminDB   string
}

// NewCluster starts a Postgres container for a test package. It returns an
// error instead of panicking when Docker provider discovery fails.
func NewCluster(ctx context.Context) (*Cluster, error) {
	return newCluster(ctx, checkDockerProvider)
}

// Main runs a package's tests against a shared Postgres container. When the
// container cannot start, for example because Docker is unavailable, it reports
// the reason and exits 0 so callers without Docker still pass.
func Main(m *testing.M, cluster **Cluster) {
	code, err := runMain(m.Run, cluster, NewCluster)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping tests: start postgres container: %v\n", err)
	}
	os.Exit(code)
}

func runMain(
	runTests func() int,
	cluster **Cluster,
	start func(context.Context) (*Cluster, error),
) (int, error) {
	started, err := start(context.Background())
	if err != nil {
		return 0, err
	}
	*cluster = started

	code := runTests()

	_ = started.Close(context.Background())
	return code, nil
}

type preflightFunc func(context.Context) error

func newCluster(ctx context.Context, preflight preflightFunc) (*Cluster, error) {
	if err := runPreflight(ctx, preflight); err != nil {
		return nil, err
	}

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

	return &Cluster{
		container: pgContainer,
		host:      host,
		port:      port.Port(),
		user:      user,
		password:  password,
		adminDB:   adminDB,
	}, nil
}

// runPreflight recovers testcontainers' provider-discovery panics and turns
// them into errors, so callers such as TestMain can skip cleanly when Docker is
// unavailable. It covers startup only, never the container's lifetime.
func runPreflight(ctx context.Context, preflight preflightFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("storage: start postgres container: check docker provider: %v", recovered)
		}
	}()

	if err := preflight(ctx); err != nil {
		return fmt.Errorf("storage: start postgres container: check docker provider: %w", err)
	}
	return nil
}

// checkDockerProvider verifies Docker is reachable before creating a container.
// Testcontainers panics from some provider-discovery paths, which runPreflight
// converts into an error.
func checkDockerProvider(ctx context.Context) error {
	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		return err
	}
	defer func() {
		_ = provider.Close()
	}()

	return provider.Health(ctx)
}

// Close terminates the Postgres test container.
func (c *Cluster) Close(ctx context.Context) error {
	if c == nil || c.container == nil {
		return nil
	}
	return c.container.Terminate(ctx)
}

// NewTestDB creates a migrated, isolated database for one test.
func (c *Cluster) NewTestDB(t testing.TB, migrations fs.FS, migrationDir string) storage.Connection {
	t.Helper()

	db, _ := c.newTestDB(t, migrations, migrationDir)
	return db
}

// NewTestDBWithConfig creates a migrated, isolated database for one test and
// returns the storage.Config that connects to the same database. Use it when a
// component needs a storage.Config, for example to boot a runtime in an
// integration test. Any additional connection created from the config belongs
// to the caller and must be closed before the test ends.
func (c *Cluster) NewTestDBWithConfig(t testing.TB, migrations fs.FS, migrationDir string) (storage.Connection, storage.Config) {
	t.Helper()

	return c.newTestDB(t, migrations, migrationDir)
}

func (c *Cluster) newTestDB(t testing.TB, migrations fs.FS, migrationDir string) (storage.Connection, storage.Config) {
	t.Helper()

	if err := validateMigrations(migrations, migrationDir); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	name := fmt.Sprintf("test_%d", time.Now().UnixNano())

	admin, err := storage.NewPostgres(ctx, storage.DefaultConfig(), storage.WithDatabaseURL(c.databaseURL(c.adminDB)))
	if err != nil {
		t.Fatalf("failed to connect to admin database: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Close()
	})

	if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	dropped := false
	t.Cleanup(func() {
		if dropped {
			return
		}
		_, _ = admin.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})

	db, err := storage.NewPostgres(ctx, storage.DefaultConfig(), storage.WithDatabaseURL(c.databaseURL(name)))
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		dropped = true
	})

	if err := storage.RunMigrations(ctx, db, migrations, migrationDir); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db, c.databaseConfig(name)
}

func (c *Cluster) databaseConfig(name string) storage.Config {
	config := storage.DefaultConfig()
	config.Host = c.host
	config.Port = c.port
	config.Name = name
	config.User = c.user
	config.Password = c.password
	config.SSLMode = "disable"
	return config
}

func (c *Cluster) databaseURL(name string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.user,
		c.password,
		c.host,
		c.port,
		name,
	)
}

func validateMigrations(migrations fs.FS, migrationDir string) error {
	if migrations == nil {
		return errors.New("migrations filesystem is required")
	}
	if strings.TrimSpace(migrationDir) == "" {
		return errors.New("migration directory is required")
	}
	return nil
}

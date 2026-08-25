package storage

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	DefaultDatabaseKind     = "postgres"
	DefaultDatabaseHost     = "127.0.0.1"
	DefaultDatabasePort     = "5432"
	DefaultDatabaseName     = "andurel"
	DefaultDatabaseUser     = "postgres"
	DefaultDatabasePassword = "postgres"
	DefaultDatabaseSSLMode  = "disable"
)

// Config contains PostgreSQL connection settings. NewPostgres starts with
// DefaultConfig and applies any options supplied by the application.
type Config struct {
	DatabaseKind string
	Host         string
	Port         string
	Name         string
	User         string
	Password     string
	SSLMode      string
}

// DefaultConfig returns development-friendly PostgreSQL defaults.
func DefaultConfig() Config {
	return Config{
		DatabaseKind: DefaultDatabaseKind,
		Host:         DefaultDatabaseHost,
		Port:         DefaultDatabasePort,
		Name:         DefaultDatabaseName,
		User:         DefaultDatabaseUser,
		Password:     DefaultDatabasePassword,
		SSLMode:      DefaultDatabaseSSLMode,
	}
}

// Validate verifies that all required connection settings are present.
func (config Config) Validate() error {
	values := map[string]string{
		"database kind": config.DatabaseKind,
		"host":          config.Host,
		"port":          config.Port,
		"database name": config.Name,
		"user":          config.User,
		"SSL mode":      config.SSLMode,
	}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("storage: %s cannot be empty", name)
		}
	}
	return nil
}

// DatabaseURL returns the PostgreSQL connection URL represented by config.
func (config Config) DatabaseURL() (string, error) {
	if err := config.Validate(); err != nil {
		return "", err
	}

	return (&url.URL{
		Scheme: strings.TrimSpace(config.DatabaseKind),
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(strings.TrimSpace(config.Host), strings.TrimSpace(config.Port)),
		Path:   "/" + strings.TrimPrefix(config.Name, "/"),
		RawQuery: url.Values{
			"sslmode": []string{config.SSLMode},
		}.Encode(),
	}).String(), nil
}

// Option configures a PostgreSQL connection.
type Option func(*postgresOptions) error

type postgresOptions struct {
	config    Config
	configURL string
	tracer    pgx.QueryTracer
}

// WithConfig replaces the default connection settings.
func WithConfig(config Config) Option {
	return func(options *postgresOptions) error {
		if err := config.Validate(); err != nil {
			return err
		}
		options.config = config
		options.configURL = ""
		return nil
	}
}

// WithDatabaseKind sets the URL scheme used for the connection.
func WithDatabaseKind(kind string) Option {
	return func(options *postgresOptions) error {
		options.config.DatabaseKind = kind
		options.configURL = ""
		return nil
	}
}

// WithHost sets the database host.
func WithHost(host string) Option {
	return func(options *postgresOptions) error { options.config.Host = host; options.configURL = ""; return nil }
}

// WithPort sets the database port.
func WithPort(port string) Option {
	return func(options *postgresOptions) error { options.config.Port = port; options.configURL = ""; return nil }
}

// WithDatabaseName sets the database name.
func WithDatabaseName(name string) Option {
	return func(options *postgresOptions) error { options.config.Name = name; options.configURL = ""; return nil }
}

// WithUser sets the database user.
func WithUser(user string) Option {
	return func(options *postgresOptions) error { options.config.User = user; options.configURL = ""; return nil }
}

// WithPassword sets the database password.
func WithPassword(password string) Option {
	return func(options *postgresOptions) error {
		options.config.Password = password
		options.configURL = ""
		return nil
	}
}

// WithSSLMode sets the PostgreSQL SSL mode.
func WithSSLMode(mode string) Option {
	return func(options *postgresOptions) error {
		options.config.SSLMode = mode
		options.configURL = ""
		return nil
	}
}

// WithDatabaseURL configures the connection from a PostgreSQL URL.
func WithDatabaseURL(databaseURL string) Option {
	return func(options *postgresOptions) error {
		if _, err := pgx.ParseConfig(databaseURL); err != nil {
			return fmt.Errorf("storage: parse database URL: %w", err)
		}
		options.configURL = databaseURL
		return nil
	}
}

// WithTracer replaces the default OpenTelemetry pgx tracer. A nil tracer
// disables database tracing.
func WithTracer(tracer pgx.QueryTracer) Option {
	return func(options *postgresOptions) error {
		options.tracer = tracer
		return nil
	}
}

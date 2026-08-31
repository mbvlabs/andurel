package storage

import (
	"crypto/tls"
	"fmt"
	"maps"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	DefaultDatabaseKind             = "postgres"
	DefaultDatabaseHost             = "127.0.0.1"
	DefaultDatabasePort             = "5432"
	DefaultDatabaseName             = "andurel"
	DefaultDatabaseUser             = "postgres"
	DefaultDatabasePassword         = "postgres"
	DefaultDatabaseSSLMode          = "disable"
	DefaultApplicationName          = "andurel"
	DefaultConnectTimeout           = 5 * time.Second
	DefaultStatementCacheCapacity   = 512
	DefaultDescriptionCacheCapacity = 512
	DefaultMaxOpenConnections       = 25
	DefaultMaxIdleConnections       = 25
	DefaultConnectionMaxLifetime    = time.Hour
	DefaultConnectionMaxIdleTime    = 30 * time.Minute
)

// Config contains PostgreSQL connection settings. NewPostgres starts with
// DefaultConfig and applies any options supplied by the application.
type Config struct {
	DatabaseKind             string
	Host                     string
	Port                     string
	Name                     string
	User                     string
	Password                 string
	SSLMode                  string
	ApplicationName          string
	ConnectTimeout           time.Duration
	StatementCacheCapacity   int
	DescriptionCacheCapacity int
	MaxOpenConnections       int
	MaxIdleConnections       int
	ConnectionMaxLifetime    time.Duration
	ConnectionMaxIdleTime    time.Duration
	OpenTelemetry            bool
}

// DefaultConfig returns development-friendly PostgreSQL defaults.
func DefaultConfig() Config {
	return Config{
		DatabaseKind:             DefaultDatabaseKind,
		Host:                     DefaultDatabaseHost,
		Port:                     DefaultDatabasePort,
		Name:                     DefaultDatabaseName,
		User:                     DefaultDatabaseUser,
		Password:                 DefaultDatabasePassword,
		SSLMode:                  DefaultDatabaseSSLMode,
		ApplicationName:          DefaultApplicationName,
		ConnectTimeout:           DefaultConnectTimeout,
		StatementCacheCapacity:   DefaultStatementCacheCapacity,
		DescriptionCacheCapacity: DefaultDescriptionCacheCapacity,
		MaxOpenConnections:       DefaultMaxOpenConnections,
		MaxIdleConnections:       DefaultMaxIdleConnections,
		ConnectionMaxLifetime:    DefaultConnectionMaxLifetime,
		ConnectionMaxIdleTime:    DefaultConnectionMaxIdleTime,
		OpenTelemetry:            true,
	}
}

// Validate verifies that all required connection settings are present.
func (config Config) Validate() error {
	for name, value := range map[string]string{
		"database kind": config.DatabaseKind,
		"host":          config.Host,
		"port":          config.Port,
		"database name": config.Name,
		"user":          config.User,
		"SSL mode":      config.SSLMode,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("storage: %s cannot be empty", name)
		}
	}
	for name, value := range map[string]int{
		"statement cache capacity":   config.StatementCacheCapacity,
		"description cache capacity": config.DescriptionCacheCapacity,
		"maximum open connections":   config.MaxOpenConnections,
		"maximum idle connections":   config.MaxIdleConnections,
	} {
		if value < 0 {
			return fmt.Errorf("storage: %s cannot be negative", name)
		}
	}
	for name, value := range map[string]time.Duration{
		"connect timeout":              config.ConnectTimeout,
		"connection maximum lifetime":  config.ConnectionMaxLifetime,
		"connection maximum idle time": config.ConnectionMaxIdleTime,
	} {
		if value < 0 {
			return fmt.Errorf("storage: %s cannot be negative", name)
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

// TelemetryConfig configures PostgreSQL OpenTelemetry instrumentation without
// exposing the underlying database driver.
type TelemetryConfig struct {
	TracerProvider         trace.TracerProvider
	MeterProvider          metric.MeterProvider
	Attributes             []attribute.KeyValue
	TrimSQLInSpanName      bool
	IncludeQueryParameters bool
}

// Option configures a PostgreSQL connection.
type Option func(*postgresOptions) error

type postgresOptions struct {
	config                   Config
	configURL                string
	telemetry                *TelemetryConfig
	telemetrySet             bool
	connectTimeout           *time.Duration
	applicationName          *string
	statementCacheCapacity   *int
	descriptionCacheCapacity *int
	runtimeParameters        map[string]string
	tlsConfig                *tls.Config
	maxOpenConnections       *int
	maxIdleConnections       *int
	connectionMaxLifetime    *time.Duration
	connectionMaxIdleTime    *time.Duration
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

// WithDatabaseURL configures the connection from a PostgreSQL URL.
func WithDatabaseURL(databaseURL string) Option {
	return func(options *postgresOptions) error {
		if strings.TrimSpace(databaseURL) == "" {
			return fmt.Errorf("storage: database URL cannot be empty")
		}
		options.configURL = databaseURL
		return nil
	}
}

// WithOpenTelemetry configures PostgreSQL tracing and metrics.
func WithOpenTelemetry(config TelemetryConfig) Option {
	return func(options *postgresOptions) error {
		options.telemetry = &config
		options.telemetrySet = true
		return nil
	}
}

// WithoutOpenTelemetry disables PostgreSQL tracing and metrics.
func WithoutOpenTelemetry() Option {
	return func(options *postgresOptions) error {
		options.telemetry = nil
		options.telemetrySet = true
		return nil
	}
}

// WithConnectTimeout sets the PostgreSQL connection timeout.
func WithConnectTimeout(timeout time.Duration) Option {
	return func(options *postgresOptions) error {
		if timeout < 0 {
			return fmt.Errorf("storage: connect timeout cannot be negative")
		}
		options.connectTimeout = &timeout
		return nil
	}
}

// WithApplicationName sets the PostgreSQL application_name runtime parameter.
func WithApplicationName(name string) Option {
	return func(options *postgresOptions) error {
		options.applicationName = &name
		return nil
	}
}

// WithStatementCacheCapacity sets the prepared statement cache capacity.
func WithStatementCacheCapacity(capacity int) Option {
	return cacheCapacityOption(
		"statement",
		capacity,
		func(options *postgresOptions) { options.statementCacheCapacity = &capacity },
	)
}

// WithDescriptionCacheCapacity sets the query description cache capacity.
func WithDescriptionCacheCapacity(capacity int) Option {
	return cacheCapacityOption(
		"description",
		capacity,
		func(options *postgresOptions) { options.descriptionCacheCapacity = &capacity },
	)
}

func cacheCapacityOption(name string, capacity int, apply func(*postgresOptions)) Option {
	return func(options *postgresOptions) error {
		if capacity < 0 {
			return fmt.Errorf("storage: %s cache capacity cannot be negative", name)
		}
		apply(options)
		return nil
	}
}

// WithRuntimeParameters merges PostgreSQL runtime parameters.
func WithRuntimeParameters(parameters map[string]string) Option {
	return func(options *postgresOptions) error {
		maps.Copy(options.runtimeParameters, parameters)
		return nil
	}
}

// WithTLSConfig replaces the TLS configuration used by PostgreSQL.
func WithTLSConfig(config *tls.Config) Option {
	return func(options *postgresOptions) error {
		if config == nil {
			return fmt.Errorf("storage: TLS config cannot be nil")
		}
		options.tlsConfig = config.Clone()
		return nil
	}
}

// WithMaxOpenConnections sets the database/sql open connection limit.
func WithMaxOpenConnections(count int) Option {
	return connectionCountOption(
		"open",
		count,
		func(options *postgresOptions) { options.maxOpenConnections = &count },
	)
}

// WithMaxIdleConnections sets the database/sql idle connection limit.
func WithMaxIdleConnections(count int) Option {
	return connectionCountOption(
		"idle",
		count,
		func(options *postgresOptions) { options.maxIdleConnections = &count },
	)
}

func connectionCountOption(name string, count int, apply func(*postgresOptions)) Option {
	return func(options *postgresOptions) error {
		if count < 0 {
			return fmt.Errorf("storage: maximum %s connections cannot be negative", name)
		}
		apply(options)
		return nil
	}
}

// WithConnectionMaxLifetime sets the maximum reusable connection lifetime.
func WithConnectionMaxLifetime(lifetime time.Duration) Option {
	return connectionDurationOption(
		"lifetime",
		lifetime,
		func(options *postgresOptions) { options.connectionMaxLifetime = &lifetime },
	)
}

// WithConnectionMaxIdleTime sets the maximum time a connection may remain idle.
func WithConnectionMaxIdleTime(idleTime time.Duration) Option {
	return connectionDurationOption(
		"idle time",
		idleTime,
		func(options *postgresOptions) { options.connectionMaxIdleTime = &idleTime },
	)
}

func connectionDurationOption(
	name string,
	duration time.Duration,
	apply func(*postgresOptions),
) Option {
	return func(options *postgresOptions) error {
		if duration < 0 {
			return fmt.Errorf("storage: connection maximum %s cannot be negative", name)
		}
		apply(options)
		return nil
	}
}

func telemetryOptions(config TelemetryConfig) []otelpgx.Option {
	var options []otelpgx.Option
	if config.TracerProvider != nil {
		options = append(options, otelpgx.WithTracerProvider(config.TracerProvider))
	}
	if config.MeterProvider != nil {
		options = append(options, otelpgx.WithMeterProvider(config.MeterProvider))
	}
	if len(config.Attributes) > 0 {
		options = append(options, otelpgx.WithAttributes(config.Attributes...))
	}
	if config.TrimSQLInSpanName {
		options = append(options, otelpgx.WithTrimSQLInSpanName())
	}
	if config.IncludeQueryParameters {
		options = append(options, otelpgx.WithIncludeQueryParameters())
	}
	return options
}

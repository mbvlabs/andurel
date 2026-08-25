package storage

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.DatabaseKind != DefaultDatabaseKind ||
		config.Host != DefaultDatabaseHost ||
		config.Port != DefaultDatabasePort ||
		config.Name != DefaultDatabaseName ||
		config.User != DefaultDatabaseUser ||
		config.Password != DefaultDatabasePassword ||
		config.SSLMode != DefaultDatabaseSSLMode {
		t.Fatalf("unexpected default config: %#v", config)
	}
}

func TestConfigDatabaseURL(t *testing.T) {
	config := Config{
		DatabaseKind: "postgres",
		Host:         "::1",
		Port:         "5432",
		Name:         "application db",
		User:         "user@example.com",
		Password:     "secret:/?",
		SSLMode:      "require",
	}

	got, err := config.DatabaseURL()
	if err != nil {
		t.Fatalf("DatabaseURL returned an error: %v", err)
	}
	want := "postgres://user%40example.com:secret%3A%2F%3F@[::1]:5432/application%20db?sslmode=require"
	if got != want {
		t.Fatalf("DatabaseURL = %q, want %q", got, want)
	}
}

func TestConfigDatabaseURLRejectsMissingValues(t *testing.T) {
	config := DefaultConfig()
	config.Host = ""
	if _, err := config.DatabaseURL(); err == nil {
		t.Fatal("DatabaseURL accepted an empty host")
	}
}

func TestPostgresOptions(t *testing.T) {
	options := postgresOptions{runtimeParameters: make(map[string]string)}
	for _, option := range []Option{
		WithApplicationName("andurel"),
		WithConnectTimeout(3 * time.Second),
		WithStatementCacheCapacity(128),
		WithDescriptionCacheCapacity(64),
		WithRuntimeParameters(map[string]string{"search_path": "public"}),
		WithMaxOpenConnections(20),
		WithMaxIdleConnections(10),
		WithConnectionMaxLifetime(time.Hour),
		WithConnectionMaxIdleTime(5 * time.Minute),
	} {
		if err := option(&options); err != nil {
			t.Fatalf("apply option: %v", err)
		}
	}

	if options.applicationName == nil || *options.applicationName != "andurel" {
		t.Fatalf("application name was not configured: %#v", options)
	}
	if options.connectTimeout == nil || *options.connectTimeout != 3*time.Second {
		t.Fatalf("connect timeout was not configured: %#v", options)
	}
	if options.statementCacheCapacity == nil || *options.statementCacheCapacity != 128 {
		t.Fatalf("statement cache capacity was not configured")
	}
	if got := options.runtimeParameters["search_path"]; got != "public" {
		t.Fatalf("search_path = %q, want public", got)
	}
	if options.maxOpenConnections == nil || *options.maxOpenConnections != 20 {
		t.Fatalf("maximum open connections was not configured")
	}
}

func TestOpenTelemetryOptions(t *testing.T) {
	options := postgresOptions{}
	if err := WithOpenTelemetry(TelemetryConfig{TrimSQLInSpanName: true})(&options); err != nil {
		t.Fatalf("WithOpenTelemetry returned an error: %v", err)
	}
	if options.telemetry == nil || !options.telemetry.TrimSQLInSpanName {
		t.Fatal("WithOpenTelemetry did not apply its configuration")
	}
	if err := WithoutOpenTelemetry()(&options); err != nil {
		t.Fatalf("WithoutOpenTelemetry returned an error: %v", err)
	}
	if options.telemetry != nil {
		t.Fatal("WithoutOpenTelemetry did not disable telemetry")
	}
}

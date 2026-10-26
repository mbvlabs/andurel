package storage

import (
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

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
	config := Config{
		DatabaseKind: "postgres",
		Host:         "",
		Port:         "5432",
		Name:         "andurel",
		User:         "postgres",
		SSLMode:      "disable",
	}
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

type testTelemetryProvider struct {
	tracer trace.TracerProvider
	meter  metric.MeterProvider
}

func (p testTelemetryProvider) TracerProvider() trace.TracerProvider { return p.tracer }
func (p testTelemetryProvider) MeterProvider() metric.MeterProvider  { return p.meter }

func TestWithTelemetryOption(t *testing.T) {
	tracer := tracenoop.NewTracerProvider()
	meter := metricnoop.NewMeterProvider()
	options := postgresOptions{}
	if err := WithTelemetry(testTelemetryProvider{tracer: tracer, meter: meter})(&options); err != nil {
		t.Fatalf("WithTelemetry returned an error: %v", err)
	}
	if options.telemetry == nil {
		t.Fatal("WithTelemetry did not configure telemetry")
	}
	if options.telemetry.TracerProvider != tracer {
		t.Fatal("WithTelemetry did not use the provider tracer")
	}
	if options.telemetry.MeterProvider != meter {
		t.Fatal("WithTelemetry did not use the provider meter")
	}

	var nilProvider TelemetryProvider
	if err := WithTelemetry(nilProvider)(&options); err != nil {
		t.Fatalf("WithTelemetry(nil) returned an error: %v", err)
	}
	if options.telemetry != nil {
		t.Fatal("WithTelemetry(nil) did not disable telemetry")
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

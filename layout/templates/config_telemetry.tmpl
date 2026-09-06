package config

import (
	"errors"
	"fmt"
	"math"

	"github.com/gosimple/slug"
	"github.com/mbvlabs/andurel/pkg/validation"
)

const (
	DefaultTelemetryServiceVersion = "1.0.0"
	DefaultTraceSampleRate         = 1.0
	DefaultTelemetryBatchSize      = 512
	DefaultTelemetryBatchTimeoutMs = 5000
)

type Telemetry struct {
	ServiceName         string
	ServiceVersion      string
	OtlpLogsEndpoint    string
	OtlpMetricsEndpoint string
	OtlpTracesEndpoint  string
	OtlpHeaders         string
	TraceSampleRate     float64
	BatchSize           int
	BatchTimeoutMs      int
}

func NewTelemetry(app App) (Telemetry, error) {
	env := newEnvironment()
	cfg := Telemetry{
		ServiceName:         env.String("TELEMETRY_SERVICE_NAME", slug.Make(app.ProjectName)),
		ServiceVersion:      env.String("TELEMETRY_SERVICE_VERSION", DefaultTelemetryServiceVersion),
		OtlpLogsEndpoint:    env.String("OTLP_LOGS_ENDPOINT", ""),
		OtlpMetricsEndpoint: env.String("OTLP_METRICS_ENDPOINT", ""),
		OtlpTracesEndpoint:  env.String("OTLP_TRACES_ENDPOINT", ""),
		OtlpHeaders:         env.String("OTLP_HEADERS", ""),
		TraceSampleRate:     env.Float64("TRACE_SAMPLE_RATE", DefaultTraceSampleRate),
		BatchSize:           env.Int("TELEMETRY_BATCH_SIZE", DefaultTelemetryBatchSize),
		BatchTimeoutMs:      env.Int("TELEMETRY_BATCH_TIMEOUT_MS", DefaultTelemetryBatchTimeoutMs),
	}

	if err := errors.Join(env.Err(), cfg.validate()); err != nil {
		return Telemetry{}, fmt.Errorf("config: telemetry: %w", err)
	}

	return cfg, nil
}

func (c Telemetry) validate() error {
	b := validation.NewBuilder()
	b.Required("ServiceName", c.ServiceName)
	b.Required("ServiceVersion", c.ServiceVersion)
	b.MinInt("BatchSize", int64(c.BatchSize), 1)
	b.MinInt("BatchTimeoutMs", int64(c.BatchTimeoutMs), 1)
	if err := b.Err(); err != nil {
		return err
	}
	if math.IsNaN(c.TraceSampleRate) || c.TraceSampleRate < 0 || c.TraceSampleRate > 1 {
		return fmt.Errorf("TRACE_SAMPLE_RATE must be between 0 and 1")
	}

	return nil
}

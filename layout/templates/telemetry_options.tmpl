package telemetry

import (
	"fmt"
	"time"
)

type Option func(*telemetryOptions) error

type telemetryOptions struct {
	serviceName    string
	serviceVersion string
	logExporters   []LogExporter
	metricExporters []MetricExporter
	traceExporters  []TraceExporter
	batchSize      int
	batchTimeout   time.Duration
	queueSize      int
	traceSampleRate float64
}

func defaultConfig() *telemetryOptions {
	return &telemetryOptions{
		serviceName:     "unknown-service",
		serviceVersion:  "0.0.0",
		batchSize:       512,
		batchTimeout:    5 * time.Second,
		queueSize:       2048,
		traceSampleRate: 1.0,
	}
}

func WithService(name, version string) Option {
	return func(c *telemetryOptions) error {
		if name == "" {
			return fmt.Errorf("service name cannot be empty")
		}
		if version == "" {
			return fmt.Errorf("service version cannot be empty")
		}
		c.serviceName = name
		c.serviceVersion = version
		return nil
	}
}

func WithLogExporters(exporters ...LogExporter) Option {
	return func(c *telemetryOptions) error {
		for i, exp := range exporters {
			if exp == nil {
				return fmt.Errorf("log exporter at index %d is nil", i)
			}
		}
		c.logExporters = exporters
		return nil
	}
}

func WithMetricExporters(exporters ...MetricExporter) Option {
	return func(c *telemetryOptions) error {
		for i, exp := range exporters {
			if exp == nil {
				return fmt.Errorf("metric exporter at index %d is nil", i)
			}
		}
		c.metricExporters = exporters
		return nil
	}
}

func WithTraceExporters(exporters ...TraceExporter) Option {
	return func(c *telemetryOptions) error {
		for i, exp := range exporters {
			if exp == nil {
				return fmt.Errorf("trace exporter at index %d is nil", i)
			}
		}
		c.traceExporters = exporters
		return nil
	}
}

func WithBatchConfig(size int, timeoutMs int, queueSize int) Option {
	return func(c *telemetryOptions) error {
		if size <= 0 {
			return fmt.Errorf("batch size must be positive, got %d", size)
		}
		if timeoutMs <= 0 {
			return fmt.Errorf("batch timeout must be positive, got %d", timeoutMs)
		}
		if queueSize <= 0 {
			return fmt.Errorf("queue size must be positive, got %d", queueSize)
		}
		c.batchSize = size
		c.batchTimeout = time.Duration(timeoutMs) * time.Millisecond
		c.queueSize = queueSize
		return nil
	}
}

func WithTraceSampleRate(rate float64) Option {
	return func(c *telemetryOptions) error {
		if rate < 0.0 || rate > 1.0 {
			return fmt.Errorf("trace sample rate must be between 0.0 and 1.0, got %f", rate)
		}
		c.traceSampleRate = rate
		return nil
	}
}

package telemetry

import (
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"
)

// Option configures telemetry exporters and sampling.
type Option func(*options) error

type otlpSettings struct {
	endpoint string
	headers  map[string]string
}

type options struct {
	console         bool
	consoleSet      bool
	logLevel        slog.Level
	otlpLogs        *otlpSettings
	otlpTraces      *otlpSettings
	otlpMetrics     *otlpSettings
	traceSampleRate float64
	batchSize       int
	batchTimeout    time.Duration
	queueSize       int
}

func defaultOptions() *options {
	return &options{
		logLevel:        slog.LevelInfo,
		traceSampleRate: 1.0,
		batchSize:       512,
		batchTimeout:    5 * time.Second,
		queueSize:       2048,
	}
}

// WithConsole writes pretty logs and completed-span lines to stdout.
func WithConsole() Option {
	return func(o *options) error {
		o.console = true
		o.consoleSet = true
		return nil
	}
}

// WithLogLevel sets the minimum severity for console and OTLP logs.
func WithLogLevel(level slog.Level) Option {
	return func(o *options) error {
		o.logLevel = level
		return nil
	}
}

// WithOTLPLogs exports logs to an OTLP HTTP endpoint.
func WithOTLPLogs(endpoint string, headers map[string]string) Option {
	return func(o *options) error {
		if strings.TrimSpace(endpoint) == "" {
			return fmt.Errorf("telemetry: OTLP logs endpoint cannot be empty")
		}
		o.otlpLogs = &otlpSettings{endpoint: endpoint, headers: cloneHeaders(headers)}
		return nil
	}
}

// WithOTLPTraces exports traces to an OTLP HTTP endpoint.
func WithOTLPTraces(endpoint string, headers map[string]string) Option {
	return func(o *options) error {
		if strings.TrimSpace(endpoint) == "" {
			return fmt.Errorf("telemetry: OTLP traces endpoint cannot be empty")
		}
		o.otlpTraces = &otlpSettings{endpoint: endpoint, headers: cloneHeaders(headers)}
		return nil
	}
}

// WithOTLPMetrics exports metrics to an OTLP HTTP endpoint.
func WithOTLPMetrics(endpoint string, headers map[string]string) Option {
	return func(o *options) error {
		if strings.TrimSpace(endpoint) == "" {
			return fmt.Errorf("telemetry: OTLP metrics endpoint cannot be empty")
		}
		o.otlpMetrics = &otlpSettings{endpoint: endpoint, headers: cloneHeaders(headers)}
		return nil
	}
}

// WithTraceSampleRate sets the trace ID ratio sampler. Rate must be in [0, 1].
func WithTraceSampleRate(rate float64) Option {
	return func(o *options) error {
		if rate < 0 || rate > 1 {
			return fmt.Errorf("telemetry: trace sample rate must be between 0 and 1, got %f", rate)
		}
		o.traceSampleRate = rate
		return nil
	}
}

// WithBatchConfig sets OTLP batch size, export timeout, and queue size.
func WithBatchConfig(size int, timeout time.Duration, queueSize int) Option {
	return func(o *options) error {
		if size <= 0 {
			return fmt.Errorf("telemetry: batch size must be positive, got %d", size)
		}
		if timeout <= 0 {
			return fmt.Errorf("telemetry: batch timeout must be positive, got %s", timeout)
		}
		if queueSize <= 0 {
			return fmt.Errorf("telemetry: queue size must be positive, got %d", queueSize)
		}
		o.batchSize = size
		o.batchTimeout = timeout
		o.queueSize = queueSize
		return nil
	}
}

// ParseHeaders parses comma-separated k=v pairs from OTLP_HEADERS.
func ParseHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)
	if headersStr == "" {
		return headers
	}
	for pair := range strings.SplitSeq(headersStr, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	maps.Copy(cloned, headers)
	return cloned
}

func (o *options) consoleLogs() bool {
	if o.consoleSet {
		return o.console
	}
	return o.otlpLogs == nil
}

func (o *options) consoleTraces() bool {
	if o.consoleSet {
		return o.console
	}
	return o.otlpTraces == nil
}

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type contextKey struct{}

// Telemetry owns log, trace, and metric providers for one process.
type Telemetry struct {
	serviceName    string
	resource       *resource.Resource
	logger         *slog.Logger
	handler        slog.Handler
	loggerProvider *sdklog.LoggerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	shutdownFuncs  []func(context.Context) error

	httpRequestsTotal metric.Int64Counter
	httpDuration      metric.Float64Histogram
	httpInFlight      metric.Int64UpDownCounter
}

// New constructs telemetry for serviceName and serviceVersion.
func New(ctx context.Context, serviceName, serviceVersion string, opts ...Option) (*Telemetry, error) {
	if strings.TrimSpace(serviceName) == "" {
		return nil, fmt.Errorf("telemetry: service name cannot be empty")
	}
	if strings.TrimSpace(serviceVersion) == "" {
		return nil, fmt.Errorf("telemetry: service version cannot be empty")
	}

	cfg := defaultOptions()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create resource: %w", err)
	}

	t := &Telemetry{
		serviceName:   serviceName,
		resource:      res,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	if err := t.initLogging(ctx, cfg); err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}
	if err := t.initTracing(ctx, cfg); err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}
	if err := t.initMetrics(ctx, cfg); err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}

	return t, nil
}

func (t *Telemetry) initLogging(ctx context.Context, cfg *options) error {
	handlers := make([]slog.Handler, 0, 2)
	if cfg.consoleLogs() {
		handlers = append(handlers, newConsoleLogHandler(cfg.logLevel))
	}
	if cfg.otlpLogs != nil {
		handler, provider, err := newOTLPLogHandler(
			ctx,
			t.resource,
			t.serviceName,
			cfg.otlpLogs.endpoint,
			cfg.otlpLogs.headers,
			cfg.batchSize,
			cfg.batchTimeout,
		)
		if err != nil {
			return err
		}
		t.loggerProvider = provider
		t.shutdownFuncs = append(t.shutdownFuncs, provider.Shutdown)
		handlers = append(handlers, handler)
	}
	if len(handlers) == 0 {
		handlers = append(handlers, newConsoleLogHandler(cfg.logLevel))
	}

	var final slog.Handler
	if len(handlers) == 1 {
		final = handlers[0]
	} else {
		final = &multiHandler{handlers: handlers}
	}
	t.handler = &traceLogHandler{handler: final}
	t.logger = slog.New(t.handler)
	return nil
}

func (t *Telemetry) initTracing(ctx context.Context, cfg *options) error {
	providerOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(t.resource),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.traceSampleRate)),
	}
	if cfg.consoleTraces() && t.logger != nil {
		providerOpts = append(providerOpts, sdktrace.WithSpanProcessor(&consoleSpanProcessor{
			logger: t.logger,
		}))
	}
	if cfg.otlpTraces != nil {
		exporter, err := newOTLPTraceExporter(ctx, cfg.otlpTraces.endpoint, cfg.otlpTraces.headers)
		if err != nil {
			return err
		}
		processor := sdktrace.NewBatchSpanProcessor(exporter,
			sdktrace.WithMaxQueueSize(cfg.queueSize),
			sdktrace.WithMaxExportBatchSize(cfg.batchSize),
			sdktrace.WithBatchTimeout(cfg.batchTimeout),
		)
		providerOpts = append(providerOpts, sdktrace.WithSpanProcessor(processor))
	}

	t.tracerProvider = sdktrace.NewTracerProvider(providerOpts...)
	t.shutdownFuncs = append(t.shutdownFuncs, t.tracerProvider.Shutdown)
	t.tracer = t.tracerProvider.Tracer(t.serviceName)
	return nil
}

func (t *Telemetry) initMetrics(ctx context.Context, cfg *options) error {
	if cfg.otlpMetrics == nil {
		return nil
	}

	exporter, err := newOTLPMetricExporter(ctx, cfg.otlpMetrics.endpoint, cfg.otlpMetrics.headers)
	if err != nil {
		return err
	}
	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.batchTimeout))
	t.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(t.resource),
		sdkmetric.WithReader(reader),
	)
	t.shutdownFuncs = append(t.shutdownFuncs, t.meterProvider.Shutdown)

	if err := runtime.Start(runtime.WithMeterProvider(t.meterProvider)); err != nil {
		return fmt.Errorf("telemetry: start runtime metrics: %w", err)
	}
	return t.initHTTPMetrics()
}

func (t *Telemetry) initHTTPMetrics() error {
	meter := t.meterProvider.Meter(t.serviceName)
	var err error
	t.httpRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("telemetry: create http_requests_total: %w", err)
	}
	t.httpDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return fmt.Errorf("telemetry: create http_request_duration_seconds: %w", err)
	}
	t.httpInFlight, err = meter.Int64UpDownCounter(
		"http_requests_in_flight",
		metric.WithDescription("Current number of HTTP requests being served"),
	)
	if err != nil {
		return fmt.Errorf("telemetry: create http_requests_in_flight: %w", err)
	}
	return nil
}

// Shutdown flushes and closes exporters.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	var shutdownErr error
	for _, shutdown := range slices.Backward(t.shutdownFuncs) {
		shutdownErr = errors.Join(shutdownErr, shutdown(ctx))
	}
	return shutdownErr
}

// TracerProvider returns the SDK tracer provider. It is never the process default.
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	if t == nil || t.tracerProvider == nil {
		return noop.NewTracerProvider()
	}
	return t.tracerProvider
}

// MeterProvider returns the SDK meter provider, or nil when metrics are disabled.
func (t *Telemetry) MeterProvider() metric.MeterProvider {
	if t == nil || t.meterProvider == nil {
		return nil
	}
	return t.meterProvider
}

// Logger returns a slog logger that writes through this telemetry pipeline.
func (t *Telemetry) Logger() *slog.Logger {
	if t == nil || t.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return t.logger
}

// Context attaches t to ctx so Start, Set, Fail, and log helpers can find it.
func (t *Telemetry) Context(ctx context.Context) context.Context {
	if t == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, t)
}

// ServiceName returns the service name passed to New.
func (t *Telemetry) ServiceName() string {
	if t == nil {
		return ""
	}
	return t.serviceName
}

// HasMetrics reports whether an OTLP metrics exporter is configured.
func (t *Telemetry) HasMetrics() bool {
	return t != nil && t.meterProvider != nil
}

// HasTracing reports whether a tracer provider is configured.
func (t *Telemetry) HasTracing() bool {
	return t != nil && t.tracerProvider != nil
}

// HasLogging reports whether a log handler is configured.
func (t *Telemetry) HasLogging() bool {
	return t != nil && t.logger != nil
}

func fromContext(ctx context.Context) *Telemetry {
	if ctx == nil {
		return nil
	}
	tel, _ := ctx.Value(contextKey{}).(*Telemetry)
	return tel
}

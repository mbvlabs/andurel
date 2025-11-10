package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"golang.org/x/sync/errgroup"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Telemetry struct {
	resource       *resource.Resource
	loggerProvider *sdklog.LoggerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	shutdownFuncs  []func(context.Context) error
	config         *telemetryOptions
}

func New(ctx context.Context, opts ...Option) (*Telemetry, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.serviceName),
			semconv.ServiceVersionKey.String(cfg.serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	t := &Telemetry{
		resource:      res,
		shutdownFuncs: make([]func(context.Context) error, 0),
		config:        cfg,
	}

	if err := t.initLogging(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize logging: %w", err)
	}

	if err := t.initMetrics(ctx); err != nil {
		_ = t.Shutdown(ctx)
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	if err := t.initTracing(ctx); err != nil {
		_ = t.Shutdown(ctx)
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	return t, nil
}

func (t *Telemetry) initLogging(ctx context.Context) error {
	if len(t.config.logExporters) == 0 {
		return nil
	}

	handlers := make([]slog.Handler, 0, len(t.config.logExporters))
	for _, exporter := range t.config.logExporters {
		handler, err := exporter.GetSlogHandler(ctx)
		if err != nil {
			return fmt.Errorf("failed to get slog handler from %s: %w", exporter.Name(), err)
		}
		handlers = append(handlers, handler)
		t.shutdownFuncs = append(t.shutdownFuncs, exporter.Shutdown)
	}

	var finalHandler slog.Handler
	if len(handlers) == 1 {
		finalHandler = handlers[0]
	} else {
		finalHandler = &multiHandler{handlers: handlers}
	}

	wrappedHandler := &traceLogHandler{handler: finalHandler}
	logger := slog.New(wrappedHandler)
	slog.SetDefault(logger)

	return nil
}

func (t *Telemetry) initMetrics(ctx context.Context) error {
	if len(t.config.metricExporters) == 0 {
		return nil
	}

	exporters := make([]sdkmetric.Exporter, 0, len(t.config.metricExporters))
	for _, exporter := range t.config.metricExporters {
		exp, err := exporter.GetSdkMetricExporter(ctx, t.resource)
		if err != nil {
			return fmt.Errorf("failed to get metric exporter from %s: %w", exporter.Name(), err)
		}
		exporters = append(exporters, exp)
		t.shutdownFuncs = append(t.shutdownFuncs, exporter.Shutdown)
	}

	opts := []sdkmetric.Option{
		sdkmetric.WithResource(t.resource),
	}

	for _, exp := range exporters {
		reader := sdkmetric.NewPeriodicReader(exp,
			sdkmetric.WithInterval(t.config.batchTimeout),
		)
		opts = append(opts, sdkmetric.WithReader(reader))
	}

	meterProvider := sdkmetric.NewMeterProvider(opts...)

	t.meterProvider = meterProvider
	t.shutdownFuncs = append(t.shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	return nil
}

func (t *Telemetry) initTracing(ctx context.Context) error {
	if len(t.config.traceExporters) == 0 {
		return nil
	}

	exporters := make([]sdktrace.SpanExporter, 0, len(t.config.traceExporters))
	for _, exporter := range t.config.traceExporters {
		exp, err := exporter.GetSpanExporter(ctx, t.resource)
		if err != nil {
			return fmt.Errorf("failed to get span exporter from %s: %w", exporter.Name(), err)
		}
		exporters = append(exporters, exp)
		t.shutdownFuncs = append(t.shutdownFuncs, exporter.Shutdown)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(t.resource),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(t.config.traceSampleRate)),
	}

	for _, exp := range exporters {
		processor := sdktrace.NewBatchSpanProcessor(exp,
			sdktrace.WithMaxQueueSize(t.config.queueSize),
			sdktrace.WithMaxExportBatchSize(t.config.batchSize),
			sdktrace.WithBatchTimeout(t.config.batchTimeout),
		)
		opts = append(opts, sdktrace.WithSpanProcessor(processor))
	}

	tracerProvider := sdktrace.NewTracerProvider(opts...)

	t.tracerProvider = tracerProvider
	t.shutdownFuncs = append(t.shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	return nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	eg := errgroup.Group{}
	for _, fn := range t.shutdownFuncs {
		fn := fn
		eg.Go(func() error {
			if err := fn(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "[telemetry] shutdown error: %v\n", err)
				return err
			}
			return nil
		})
	}
	return eg.Wait()
}

func (t *Telemetry) HealthCheck(ctx context.Context) error {
	if len(t.config.logExporters) == 0 && len(t.config.metricExporters) == 0 && len(t.config.traceExporters) == 0 {
		return fmt.Errorf("no exporters configured")
	}
	return nil
}

func (t *Telemetry) HasMetrics() bool {
	return t.meterProvider != nil
}

func (t *Telemetry) HasTracing() bool {
	return t.tracerProvider != nil
}

func (t *Telemetry) HasLogging() bool {
	return len(t.config.logExporters) > 0
}

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil {
			fmt.Fprintf(os.Stderr, "[telemetry] handler error: %v\n", err)
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

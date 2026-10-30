package telemetry

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	messages := make([]string, 0, len(h.records))
	for _, record := range h.records {
		messages = append(messages, record.Message)
	}
	return messages
}

func captureDefaultLogger(t *testing.T) *captureHandler {
	t.Helper()
	capture := &captureHandler{}
	previous := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return capture
}

func TestLogFallsBackToDefaultLoggerWithoutTelemetry(t *testing.T) {
	capture := captureDefaultLogger(t)

	Warn(context.Background(), "fallback warning", "kind", "test")

	if !slices.Contains(capture.messages(), "fallback warning") {
		t.Fatalf("slog.Default did not receive the record: %v", capture.messages())
	}
}

func TestLogUsesTelemetryLoggerInsteadOfDefault(t *testing.T) {
	defaultCapture := captureDefaultLogger(t)
	telemetryCapture := &captureHandler{}
	tel := &Telemetry{logger: slog.New(telemetryCapture)}

	Info(tel.Context(context.Background()), "telemetry info", "kind", "test")

	if !slices.Contains(telemetryCapture.messages(), "telemetry info") {
		t.Fatalf("telemetry logger did not receive the record: %v", telemetryCapture.messages())
	}
	if messages := defaultCapture.messages(); len(messages) != 0 {
		t.Fatalf("slog.Default received telemetry records: %v", messages)
	}
}

func TestLogRecordsSpanEventWithoutTelemetry(t *testing.T) {
	captureDefaultLogger(t)
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	ctx, span := provider.Tracer("test").Start(context.Background(), "operation")

	Warn(ctx, "span event", "link_id", "l-1")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	if len(spans[0].Events) != 1 || spans[0].Events[0].Name != "span event" {
		t.Fatalf("span events = %#v, want one %q event", spans[0].Events, "span event")
	}
}

func TestLogRecordsSpanEventWithTelemetry(t *testing.T) {
	capture := &captureHandler{}
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	tel := &Telemetry{
		logger:         slog.New(capture),
		tracerProvider: provider,
		tracer:         provider.Tracer("test"),
	}

	ctx, span := Start(tel.Context(context.Background()), "operation")
	Warn(ctx, "telemetry span event", "link_id", "l-1")
	span.End()

	if !slices.Contains(capture.messages(), "telemetry span event") {
		t.Fatalf("telemetry logger did not receive the record: %v", capture.messages())
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	if len(spans[0].Events) != 1 || spans[0].Events[0].Name != "telemetry span event" {
		t.Fatalf("span events = %#v, want one %q event", spans[0].Events, "telemetry span event")
	}
}

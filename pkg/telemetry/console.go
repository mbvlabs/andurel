package telemetry

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func newConsoleLogHandler(level slog.Level) slog.Handler {
	return tint.NewTextHandler(os.Stdout, &tint.Options{
		Level:      level,
		TimeFormat: "15:04:05",
		AddSource:  true,
	})
}

type traceLogHandler struct {
	handler slog.Handler
}

func (h *traceLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *traceLogHandler) Handle(ctx context.Context, record slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.handler.Handle(ctx, record)
}

func (h *traceLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceLogHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *traceLogHandler) WithGroup(name string) slog.Handler {
	return &traceLogHandler{handler: h.handler.WithGroup(name)}
}

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range m.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range m.handlers {
		if err := handler.Handle(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, handler := range m.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, handler := range m.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

type consoleSpanProcessor struct {
	logger *slog.Logger
}

func (p *consoleSpanProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

func (p *consoleSpanProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	if p == nil || p.logger == nil {
		return
	}
	if !p.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	status := "ok"
	if span.Status().Code == codes.Error {
		status = "error"
	}
	attrs := make([]any, 0, 8)
	attrs = append(attrs,
		"span", span.Name(),
		"duration", span.EndTime().Sub(span.StartTime()).String(),
		"status", status,
	)
	if parent := span.Parent(); parent.IsValid() {
		attrs = append(attrs, "parent_span_id", parent.SpanID().String())
	}
	for _, attr := range span.Attributes() {
		attrs = append(attrs, string(attr.Key), attr.Value.AsInterface())
	}
	record := slog.NewRecord(time.Now(), slog.LevelDebug, "span complete", 0)
	record.Add(attrs...)
	_ = p.logger.Handler().Handle(context.Background(), record)
}

func (p *consoleSpanProcessor) Shutdown(context.Context) error { return nil }

func (p *consoleSpanProcessor) ForceFlush(context.Context) error { return nil }

var _ sdktrace.SpanProcessor = (*consoleSpanProcessor)(nil)

var _ slog.Handler = (*traceLogHandler)(nil)

var _ slog.Handler = (*multiHandler)(nil)

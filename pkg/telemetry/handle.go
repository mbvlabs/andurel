package telemetry

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var noopTracer = noop.NewTracerProvider().Tracer("andurel")

// From starts a named span from an *http.Request and writes the child context
// back onto the request so later Set/Info/Error calls see it. Add attributes
// with Set. Handlers under anduin receive the request through anduin.Request.
func From(req *http.Request, name string) (context.Context, Span) {
	ctx := requestContext(req)
	ctx, span := startSpan(ctx, name, nil)
	attachRequest(req, ctx)
	return ctx, span
}

// Start starts a span named name with slog-shaped attributes.
func Start(ctx context.Context, name string, args ...any) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	return startSpan(ctx, name, args)
}

func startSpan(ctx context.Context, name string, args []any) (context.Context, Span) {
	tel := fromContext(ctx)
	tracer := noopTracer
	if tel != nil && tel.tracer != nil {
		tracer = tel.tracer
	}
	opts := []trace.SpanStartOption{}
	if len(args) > 0 {
		opts = append(opts, trace.WithAttributes(attrsFromArgs(args)...))
	}
	ctx, span := tracer.Start(ctx, name, opts...)
	if tel != nil {
		ctx = tel.Context(ctx)
	}
	return ctx, Span{span: span}
}

// Set adds slog-shaped attributes to the current span on ctx.
func Set(ctx context.Context, args ...any) {
	if ctx == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	span.SetAttributes(attrsFromArgs(args)...)
}

// Fail records err on the current span and returns err.
func Fail(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx == nil {
		return err
	}
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// Debug logs at debug level.
//
//go:noinline
func Debug(ctx context.Context, msg string, args ...any) {
	log(ctx, slog.LevelDebug, msg, args...)
}

// Info logs at info level.
//
//go:noinline
func Info(ctx context.Context, msg string, args ...any) {
	log(ctx, slog.LevelInfo, msg, args...)
}

// Warn logs at warn level.
//
//go:noinline
func Warn(ctx context.Context, msg string, args ...any) {
	log(ctx, slog.LevelWarn, msg, args...)
}

// Error logs at error level.
//
//go:noinline
func Error(ctx context.Context, msg string, args ...any) {
	log(ctx, slog.LevelError, msg, args...)
}

//go:noinline
func log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	tel := fromContext(ctx)
	if tel == nil || tel.logger == nil {
		return
	}
	if tel.logger.Enabled(ctx, level) {
		var pcs [1]uintptr
		// Skip Callers, log, and the exported wrapper so source is the app caller.
		runtime.Callers(3, pcs[:])
		record := slog.NewRecord(time.Now(), level, msg, pcs[0])
		record.Add(args...)
		_ = tel.logger.Handler().Handle(ctx, record)
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	span.AddEvent(msg, trace.WithAttributes(attrsFromArgs(args)...))
	if err := errorFromArgs(args); err != nil {
		span.RecordError(err)
		if level >= slog.LevelError {
			span.SetStatus(codes.Error, err.Error())
		}
	}
}

func requestContext(req *http.Request) context.Context {
	if req == nil {
		return context.Background()
	}
	if ctx := req.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func attachRequest(req *http.Request, ctx context.Context) {
	if req == nil {
		return
	}
	*req = *req.WithContext(ctx)
}

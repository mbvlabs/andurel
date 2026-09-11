package telemetry

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// WrapHandler instruments h with otelhttp using the supplied tracer provider.
func WrapHandler(name string, h http.Handler, tp trace.TracerProvider) http.Handler {
	opts := []otelhttp.Option{}
	if tp != nil {
		opts = append(opts, otelhttp.WithTracerProvider(tp))
	}
	return otelhttp.NewHandler(h, name, opts...)
}

// RecordHTTPRequest records HTTP metrics when metrics are enabled.
func RecordHTTPRequest(
	ctx context.Context,
	tel *Telemetry,
	method string,
	route string,
	status int,
	duration time.Duration,
) {
	if tel == nil || !tel.HasMetrics() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("route", route),
		attribute.Int("status_code", status),
	}
	if tel.httpRequestsTotal != nil {
		tel.httpRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if tel.httpDuration != nil {
		tel.httpDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}

// AddHTTPInFlight increments the in-flight HTTP request counter.
func AddHTTPInFlight(ctx context.Context, tel *Telemetry, delta int64) {
	if tel == nil || tel.httpInFlight == nil {
		return
	}
	tel.httpInFlight.Add(ctx, delta)
}

// SetHTTPRoute sets the HTTP route attribute on the current span.
func SetHTTPRoute(ctx context.Context, route string) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() || route == "" {
		return
	}
	span.SetAttributes(semconv.HTTPRoute(route))
}

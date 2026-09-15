package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// WrapHandler instruments h with otelhttp using this process's tracer and meter
// providers. HTTP server metrics come from otelhttp, not a parallel Andurel API.
func WrapHandler(name string, h http.Handler, tel *Telemetry) http.Handler {
	opts := []otelhttp.Option{}
	if tel != nil {
		opts = append(opts, otelhttp.WithTracerProvider(tel.TracerProvider()))
		if mp := tel.MeterProvider(); mp != nil {
			opts = append(opts, otelhttp.WithMeterProvider(mp))
		}
	}
	return otelhttp.NewHandler(h, name, opts...)
}

// SetHTTPRoute sets http.route on the current span and on otelhttp metric labels.
func SetHTTPRoute(ctx context.Context, route string) {
	if ctx == nil || route == "" {
		return
	}
	attr := semconv.HTTPRoute(route)
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		span.SetAttributes(attr)
	}
	if labeler, ok := otelhttp.LabelerFromContext(ctx); ok {
		labeler.Add(attr)
	}
}

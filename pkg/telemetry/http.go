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
// otelhttp records the raw url.path on the server span before a router matches
// the request; call SetHTTPRoute once the route is known to replace it with the
// route template. Use anduin.WithTelemetry to get that wiring automatically.
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

// redactedURLPath replaces url.path when no route matched. Server
// instrumentation records url.path before routing, so an unmatched capability
// URL would otherwise carry its token into traces.
const redactedURLPath = "/[redacted]"

// SetHTTPRoute records route on the current span as http.route and replaces the
// raw url.path recorded by otelhttp with the same route template, so a secret
// path parameter never reaches a trace exporter. The route also labels otelhttp
// metrics. An empty route means no route matched: the span keeps no http.route
// and its url.path becomes a redaction placeholder. otelhttp's server
// instrumentation records neither url.full nor url.query.
func SetHTTPRoute(ctx context.Context, route string) {
	if ctx == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	valid := span.SpanContext().IsValid()
	if route == "" {
		if valid {
			span.SetAttributes(semconv.URLPath(redactedURLPath))
		}
		return
	}
	attr := semconv.HTTPRoute(route)
	if valid {
		span.SetAttributes(attr, semconv.URLPath(route))
	}
	if labeler, ok := otelhttp.LabelerFromContext(ctx); ok {
		labeler.Add(attr)
	}
}

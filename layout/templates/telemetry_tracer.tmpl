package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type TraceExporter interface {
	Name() string
	GetSpanExporter(ctx context.Context, res *resource.Resource) (sdktrace.SpanExporter, error)
	Shutdown(ctx context.Context) error
}

func GetTracer(serviceName string) trace.Tracer {
	return otel.Tracer(serviceName)
}

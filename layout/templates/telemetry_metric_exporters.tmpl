package telemetry

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

type OtlpHttpMetricExporter struct {
	endpoint string
	insecure bool
	headers  map[string]string
	exporter sdkmetric.Exporter
}

func NewOtlpMetricExporter(endpoint string, headers map[string]string) *OtlpHttpMetricExporter {
	if endpoint == "" {
		return nil
	}
	return &OtlpHttpMetricExporter{
		endpoint: endpoint,
		insecure: false,
		headers:  headers,
		exporter: nil,
	}
}

func NewOtlpMetricExporterInsecure(endpoint string, headers map[string]string) *OtlpHttpMetricExporter {
	if endpoint == "" {
		return nil
	}
	return &OtlpHttpMetricExporter{
		endpoint: endpoint,
		insecure: true,
		headers:  headers,
		exporter: nil,
	}
}

func (o *OtlpHttpMetricExporter) Name() string {
	return "otlp-http-metrics"
}

func (o *OtlpHttpMetricExporter) GetSdkMetricExporter(ctx context.Context, res *resource.Resource) (sdkmetric.Exporter, error) {
	endpoint := strings.TrimPrefix(o.endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpoint),
	}

	if o.insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	} else {
		opts = append(opts, otlpmetrichttp.WithTLSClientConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))
	}

	if len(o.headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(o.headers))
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP HTTP metric exporter: %w", err)
	}

	o.exporter = exporter
	return exporter, nil
}

func (o *OtlpHttpMetricExporter) Shutdown(ctx context.Context) error {
	if o.exporter != nil {
		if err := o.exporter.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown OTLP HTTP metric exporter: %w", err)
		}
	}
	return nil
}

var _ MetricExporter = (*OtlpHttpMetricExporter)(nil)

type noopSdkMetricExporter struct{}

func (n *noopSdkMetricExporter) Aggregation(sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return nil
}

func (n *noopSdkMetricExporter) Export(context.Context, *metricdata.ResourceMetrics) error {
	return nil
}

func (n *noopSdkMetricExporter) ForceFlush(context.Context) error {
	return nil
}

func (n *noopSdkMetricExporter) Shutdown(context.Context) error {
	return nil
}

func (n *noopSdkMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality {
	return 0
}

var _ sdkmetric.Exporter = (*noopSdkMetricExporter)(nil)

type NoopMetricExporter struct{}

func NewNoopMetricExporter() *NoopMetricExporter {
	return &NoopMetricExporter{}
}

func (n *NoopMetricExporter) Name() string {
	return "noop-metrics"
}

func (n *NoopMetricExporter) GetSdkMetricExporter(ctx context.Context, res *resource.Resource) (sdkmetric.Exporter, error) {
	return &noopSdkMetricExporter{}, nil
}

func (n *NoopMetricExporter) Shutdown(ctx context.Context) error {
	return nil
}

var _ MetricExporter = (*NoopMetricExporter)(nil)

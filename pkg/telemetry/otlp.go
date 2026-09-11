package telemetry

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func newOTLPLogHandler(
	ctx context.Context,
	res *resource.Resource,
	serviceName string,
	endpoint string,
	headers map[string]string,
	batchSize int,
	batchTimeout time.Duration,
) (slog.Handler, *sdklog.LoggerProvider, error) {
	opts, err := otlpLogHTTPOptions(endpoint, headers)
	if err != nil {
		return nil, nil, err
	}
	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: create OTLP log exporter: %w", err)
	}
	processor := sdklog.NewBatchProcessor(exporter,
		sdklog.WithExportMaxBatchSize(batchSize),
		sdklog.WithExportInterval(batchTimeout),
	)
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)
	return otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(provider)), provider, nil
}

func newOTLPTraceExporter(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
) (sdktrace.SpanExporter, error) {
	opts := otlpTraceHTTPOptions(endpoint, headers)
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create OTLP trace exporter: %w", err)
	}
	return exporter, nil
}

func newOTLPMetricExporter(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
) (sdkmetric.Exporter, error) {
	opts := otlpMetricHTTPOptions(endpoint, headers)
	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create OTLP metric exporter: %w", err)
	}
	return exporter, nil
}

func otlpLogHTTPOptions(endpoint string, headers map[string]string) ([]otlploghttp.Option, error) {
	opts := []otlploghttp.Option{otlploghttp.WithHeaders(headers)}
	if strings.Contains(endpoint, "://") {
		endpointURL, err := url.Parse(endpoint)
		if err != nil || endpointURL.Host == "" ||
			(endpointURL.Scheme != "http" && endpointURL.Scheme != "https") {
			return nil, fmt.Errorf("telemetry: invalid OTLP HTTP log endpoint %q", endpoint)
		}
		opts = append(opts, otlploghttp.WithEndpointURL(endpoint))
		if endpointURL.Path == "" || endpointURL.Path == "/" {
			opts = append(opts, otlploghttp.WithURLPath("/v1/logs"))
		}
		if endpointURL.Scheme == "http" {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		return opts, nil
	}
	opts = append(opts, otlploghttp.WithEndpoint(endpoint))
	return opts, nil
}

func otlpTraceHTTPOptions(endpoint string, headers map[string]string) []otlptracehttp.Option {
	insecure, host := splitOTLPEndpoint(endpoint)
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(host)}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	} else {
		opts = append(opts, otlptracehttp.WithTLSClientConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))
	}
	if len(headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(headers))
	}
	return opts
}

func otlpMetricHTTPOptions(endpoint string, headers map[string]string) []otlpmetrichttp.Option {
	insecure, host := splitOTLPEndpoint(endpoint)
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(host)}
	if insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	} else {
		opts = append(opts, otlpmetrichttp.WithTLSClientConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))
	}
	if len(headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(headers))
	}
	return opts
}

func splitOTLPEndpoint(endpoint string) (insecure bool, host string) {
	insecure = strings.HasPrefix(endpoint, "http://")
	host = strings.TrimPrefix(endpoint, "http://")
	host = strings.TrimPrefix(host, "https://")
	return insecure, host
}

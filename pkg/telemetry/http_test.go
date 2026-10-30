package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const secretResetPath = "/password/live-SECRET-token/edit"

// startAttributeRecorder captures the attributes a processor sees before the
// span ends. A secret here is already visible to OnStart processors even when
// the completed span is redacted later.
type startAttributeRecorder struct {
	attributes [][]attribute.KeyValue
}

func (r *startAttributeRecorder) OnStart(
	_ context.Context,
	span sdktrace.ReadWriteSpan,
) {
	r.attributes = append(r.attributes, span.Attributes())
}

func (r *startAttributeRecorder) OnEnd(sdktrace.ReadOnlySpan) {}

func (r *startAttributeRecorder) Shutdown(context.Context) error { return nil }

func (r *startAttributeRecorder) ForceFlush(context.Context) error { return nil }

func TestWrapHandlerRedactsPathWithoutSetHTTPRoute(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	useTracerProvider(t, provider)

	handler := WrapHandler("direct-handler", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}), nil)

	request := httptest.NewRequest(http.MethodGet, secretResetPath, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	assertSpanPath(t, "completed", spans[0].Attributes, redactedURLPath)
	assertAttributesOmitSecret(t, "completed", spans[0].Attributes)
}

func TestWrapHandlerRedactsPathFromOnStart(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	startRecorder := &startAttributeRecorder{}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSpanProcessor(startRecorder),
	)
	useTracerProvider(t, provider)

	handler := WrapHandler("on-start-handler", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}), nil)

	request := httptest.NewRequest(http.MethodGet, secretResetPath, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if len(startRecorder.attributes) != 1 {
		t.Fatalf("OnStart saw %d spans, want 1", len(startRecorder.attributes))
	}
	assertSpanPath(t, "OnStart", startRecorder.attributes[0], redactedURLPath)
	assertAttributesOmitSecret(t, "OnStart", startRecorder.attributes[0])

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	assertSpanPath(t, "completed", spans[0].Attributes, redactedURLPath)
	assertAttributesOmitSecret(t, "completed", spans[0].Attributes)
}

func TestWrapHandlerKeepsMatchedRouteAfterSetHTTPRoute(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	useTracerProvider(t, provider)

	const route = "/password/{token}/edit"
	handler := WrapHandler("routed-handler", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		SetHTTPRoute(request.Context(), route)
		writer.WriteHeader(http.StatusNoContent)
	}), nil)

	request := httptest.NewRequest(http.MethodGet, secretResetPath, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	assertSpanPath(t, "completed", spans[0].Attributes, route)
	assertAttribute(t, spans[0].Attributes, "http.route", route)
	assertAttributesOmitSecret(t, "completed", spans[0].Attributes)
}

func useTracerProvider(t *testing.T, provider *sdktrace.TracerProvider) {
	t.Helper()

	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown tracer provider: %v", err)
		}
	})
}

func assertSpanPath(t *testing.T, label string, attributes []attribute.KeyValue, want string) {
	t.Helper()

	assertAttribute(t, attributes, "url.path", want)
}

func assertAttribute(t *testing.T, attributes []attribute.KeyValue, key, want string) {
	t.Helper()

	for _, attr := range attributes {
		if string(attr.Key) != key {
			continue
		}
		if attr.Value.AsString() != want {
			t.Fatalf("%s attribute = %q, want %q", key, attr.Value.AsString(), want)
		}
		return
	}
	t.Fatalf("missing %s attribute in %v", key, attributes)
}

func assertAttributesOmitSecret(t *testing.T, label string, attributes []attribute.KeyValue) {
	t.Helper()

	for _, attr := range attributes {
		value := attr.Value.Emit()
		if strings.Contains(value, "SECRET-token") {
			t.Fatalf("%s attribute %s leaks the path token: %q", label, attr.Key, value)
		}
	}
}

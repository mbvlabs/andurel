package anduin

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mbvlabs/andurel/pkg/routing"
	"github.com/mbvlabs/andurel/pkg/telemetry"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

type otlpCollector struct {
	server *httptest.Server

	mu      sync.Mutex
	traces  [][]byte
	metrics [][]byte
}

func newOTLPCollector(t *testing.T) *otlpCollector {
	t.Helper()
	collector := &otlpCollector{}
	collector.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read OTLP request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		collector.mu.Lock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/traces"):
			collector.traces = append(collector.traces, body)
		case strings.HasSuffix(r.URL.Path, "/v1/metrics"):
			collector.metrics = append(collector.metrics, body)
		}
		collector.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.server.Close)
	return collector
}

func (c *otlpCollector) payloads() (traces, metrics [][]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte(nil), c.traces...), append([][]byte(nil), c.metrics...)
}

func TestTelemetryOmitsSecretPathFromSpansAndMetrics(t *testing.T) {
	const (
		secretToken    = "preview-SECRET-token"
		queryToken     = "query-SECRET-token"
		unmatchedToken = "unmatched-SECRET-token"
		route          = "/preview/{token}/release-notes"
	)

	collector := newOTLPCollector(t)
	tel, err := telemetry.New(t.Context(), "anduin-test", "test",
		telemetry.WithOTLPTraces(collector.server.URL, nil),
		telemetry.WithOTLPMetrics(collector.server.URL, nil),
		telemetry.WithLogLevel(slog.LevelError),
	)
	if err != nil {
		t.Fatalf("telemetry.New: %v", err)
	}

	router := NewRouter(WithTelemetry(tel))
	router.Get("/preview/:token/release-notes", func(context.Context) error { return nil })
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/preview/" + secretToken + "/release-notes?access=" + queryToken)
	if err != nil {
		t.Fatalf("matched GET: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("matched status = %d, want 200", response.StatusCode)
	}

	response, err = http.Get(server.URL + "/preview/" + unmatchedToken + "/missing")
	if err != nil {
		t.Fatalf("unmatched GET: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unmatched status = %d, want 404", response.StatusCode)
	}

	if err := tel.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown telemetry: %v", err)
	}

	traces, metrics := collector.payloads()
	if len(traces) == 0 {
		t.Fatal("no trace payload was exported")
	}
	if len(metrics) == 0 {
		t.Fatal("no metric payload was exported")
	}
	for _, payload := range traces {
		assertPayloadOmits(t, "trace", payload, secretToken, queryToken, unmatchedToken)
	}
	for _, payload := range metrics {
		assertPayloadOmits(t, "metric", payload, secretToken, queryToken, unmatchedToken)
		if !bytes.Contains(payload, []byte(route)) {
			t.Fatalf("metric payload has no %q route label", route)
		}
	}

	labels := decodeMetricLabels(t, metrics)
	var routeLabelSeen bool
	for _, label := range labels {
		for key, value := range label.attributes {
			if strings.Contains(value, secretToken) || strings.Contains(value, queryToken) ||
				strings.Contains(value, unmatchedToken) {
				t.Fatalf("metric %q label %q leaks a secret: %q", label.name, key, value)
			}
		}
		if label.name == "http.server.request.duration" && label.attributes["http.route"] == route {
			routeLabelSeen = true
		}
	}
	if !routeLabelSeen {
		t.Fatalf("no http.server.request.duration label recorded http.route=%q", route)
	}

	spans := decodeSpans(t, traces)
	var matched, unmatched bool
	for _, span := range spans {
		if strings.Contains(span.name, secretToken) || strings.Contains(span.name, unmatchedToken) {
			t.Fatalf("span name leaks a secret path: %q", span.name)
		}
		for key, value := range span.attributes {
			if strings.Contains(value, secretToken) || strings.Contains(value, queryToken) ||
				strings.Contains(value, unmatchedToken) {
				t.Fatalf("span attribute %q leaks a secret: %q", key, value)
			}
		}
		switch span.attributes["http.route"] {
		case route:
			matched = true
			if span.attributes["url.path"] != route {
				t.Fatalf("matched url.path = %q, want %q", span.attributes["url.path"], route)
			}
		case "":
			if span.attributes["url.path"] != "/[redacted]" {
				t.Fatalf("unmatched url.path = %q, want the redaction placeholder", span.attributes["url.path"])
			}
			unmatched = true
		}
	}
	if !matched {
		t.Fatalf("no span recorded http.route=%q", route)
	}
	if !unmatched {
		t.Fatal("no unmatched span was exported")
	}
}

func TestTelemetryOmitsSecretPathWhenHandlerPanics(t *testing.T) {
	const (
		secretToken = "panic-SECRET-token"
		route       = "/preview/{token}/panic"
	)

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	t.Cleanup(func() { slog.SetDefault(previous) })

	collector := newOTLPCollector(t)
	tel, err := telemetry.New(t.Context(), "anduin-test", "test",
		telemetry.WithOTLPTraces(collector.server.URL, nil),
		telemetry.WithOTLPMetrics(collector.server.URL, nil),
		telemetry.WithLogLevel(slog.LevelError),
	)
	if err != nil {
		t.Fatalf("telemetry.New: %v", err)
	}

	router := NewRouter()
	// Recover outside Telemetry forces the span redaction to run while the
	// panic unwinds through the telemetry middleware.
	router.Use(Recover(), Telemetry(tel))
	router.Get("/preview/:token/panic", func(context.Context) error {
		panic("boom")
	})
	server := httptest.NewServer(telemetry.WrapHandler("http", router.Handler(), tel))
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/preview/" + secretToken + "/panic")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.StatusCode)
	}

	if err := tel.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown telemetry: %v", err)
	}

	traces, _ := collector.payloads()
	if len(traces) == 0 {
		t.Fatal("no trace payload was exported")
	}
	for _, payload := range traces {
		assertPayloadOmits(t, "trace", payload, secretToken)
	}
	var found bool
	for _, span := range decodeSpans(t, traces) {
		if span.attributes["http.route"] == route {
			found = true
			if span.attributes["url.path"] != route {
				t.Fatalf("url.path = %q, want %q", span.attributes["url.path"], route)
			}
		}
	}
	if !found {
		t.Fatalf("no span recorded http.route=%q", route)
	}
}

type exportedSpan struct {
	name       string
	attributes map[string]string
}

type exportedMetric struct {
	name       string
	attributes map[string]string
}

func decodeMetricLabels(t *testing.T, payloads [][]byte) []exportedMetric {
	t.Helper()
	var labels []exportedMetric
	for _, payload := range payloads {
		var request collectormetrics.ExportMetricsServiceRequest
		if err := proto.Unmarshal(payload, &request); err != nil {
			t.Fatalf("decode metric payload: %v", err)
		}
		for _, resource := range request.GetResourceMetrics() {
			for _, scope := range resource.GetScopeMetrics() {
				for _, metric := range scope.GetMetrics() {
					for _, attributes := range metricDataPointAttributes(metric) {
						labels = append(labels, exportedMetric{
							name:       metric.GetName(),
							attributes: attributes,
						})
					}
				}
			}
		}
	}
	return labels
}

func metricDataPointAttributes(metric *metricpb.Metric) []map[string]string {
	var points []map[string]string
	switch data := metric.GetData().(type) {
	case *metricpb.Metric_Gauge:
		for _, point := range data.Gauge.GetDataPoints() {
			points = append(points, attributeMap(point.GetAttributes()))
		}
	case *metricpb.Metric_Sum:
		for _, point := range data.Sum.GetDataPoints() {
			points = append(points, attributeMap(point.GetAttributes()))
		}
	case *metricpb.Metric_Histogram:
		for _, point := range data.Histogram.GetDataPoints() {
			points = append(points, attributeMap(point.GetAttributes()))
		}
	}
	return points
}

func attributeMap(attributes []*commonpb.KeyValue) map[string]string {
	values := make(map[string]string, len(attributes))
	for _, attr := range attributes {
		values[attr.GetKey()] = attr.GetValue().GetStringValue()
	}
	return values
}

func decodeSpans(t *testing.T, payloads [][]byte) []exportedSpan {
	t.Helper()
	var spans []exportedSpan
	for _, payload := range payloads {
		var request collectortrace.ExportTraceServiceRequest
		if err := proto.Unmarshal(payload, &request); err != nil {
			t.Fatalf("decode trace payload: %v", err)
		}
		for _, resource := range request.GetResourceSpans() {
			for _, scope := range resource.GetScopeSpans() {
				for _, span := range scope.GetSpans() {
					spans = append(spans, exportedSpan{
						name:       span.GetName(),
						attributes: attributeMap(span.GetAttributes()),
					})
				}
			}
		}
	}
	return spans
}

func assertPayloadOmits(t *testing.T, kind string, payload []byte, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if bytes.Contains(payload, []byte(secret)) {
			t.Fatalf("%s payload leaks %q", kind, secret)
		}
	}
}

// TestTelemetryRedactsPathsThatNeverReachSetHTTPRoute covers the transport paths
// that complete without a matched route: handler redirects, ServeMux redirects,
// 404, 405, unknown hosts, panics, and a middleware rejection before Telemetry.
// The start-option placeholder must protect all of them; a matched route still
// replaces it with the template.
func TestTelemetryRedactsPathsThatNeverReachSetHTTPRoute(t *testing.T) {
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	t.Cleanup(func() { slog.SetDefault(previous) })

	collector := newOTLPCollector(t)
	tel, err := telemetry.New(t.Context(), "anduin-test", "test",
		telemetry.WithOTLPTraces(collector.server.URL, nil),
		telemetry.WithOTLPMetrics(collector.server.URL, nil),
		telemetry.WithLogLevel(slog.LevelError),
	)
	if err != nil {
		t.Fatalf("telemetry.New: %v", err)
	}

	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "primary.example"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}
	router := NewRouter(WithHosts(hosts), WithTelemetry(tel))
	router.Use(Recover())
	router.Get("/preview/:token/notes", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "ok")
	})
	router.Get("/preview/:token/go", func(ctx context.Context) error {
		return Redirect(ctx, "/elsewhere", http.StatusSeeOther)
	})
	router.Get("/preview/:token/tree/*", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, "tree")
	})
	router.Get("/preview/:token/panic", func(context.Context) error {
		panic("boom")
	})
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	do := func(method, path, host string) *http.Response {
		t.Helper()
		request, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatalf("new request %s %s: %v", method, path, err)
		}
		if host != "" {
			request.Host = host
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		response.Body.Close()
		return response
	}

	responses := []struct {
		label  string
		method string
		path   string
		host   string
		status int
	}{
		{"matched", http.MethodGet, "/preview/matched-SECRET-token/notes", "", http.StatusOK},
		{"handler redirect", http.MethodGet, "/preview/redirect-SECRET-token/go", "", http.StatusSeeOther},
		{"serve mux redirect", http.MethodGet, "/preview/mux-SECRET-token/tree", "", http.StatusTemporaryRedirect},
		{"not found", http.MethodGet, "/preview/missing-SECRET-token/nope", "", http.StatusNotFound},
		{"method not allowed", http.MethodPost, "/preview/method-SECRET-token/notes", "", http.StatusMethodNotAllowed},
		{"unknown host", http.MethodGet, "/preview/host-SECRET-token/notes", "unknown.example", http.StatusOK},
		{"panic", http.MethodGet, "/preview/panic-SECRET-token/panic", "", http.StatusInternalServerError},
	}
	for _, tc := range responses {
		t.Run(tc.label, func(t *testing.T) {
			response := do(tc.method, tc.path, tc.host)
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.status)
			}
		})
	}

	// A rejection before Telemetry leaves the span without a matched route. The
	// initial placeholder is the only protection on this path.
	rejecting := NewRouter()
	rejecting.Use(
		func(Handler) Handler {
			return func(context.Context) error {
				return &HTTPError{Status: http.StatusForbidden, Message: "rejected"}
			}
		},
		Telemetry(tel),
	)
	rejecting.Get("/preview/:token/reject", func(context.Context) error { return nil })
	rejectingServer := httptest.NewServer(telemetry.WrapHandler("http", rejecting.Handler(), tel))
	t.Cleanup(rejectingServer.Close)
	request, err := http.NewRequest(
		http.MethodGet,
		rejectingServer.URL+"/preview/reject-SECRET-token/reject",
		nil,
	)
	if err != nil {
		t.Fatalf("new rejection request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("rejection request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("rejection status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}

	if err := tel.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown telemetry: %v", err)
	}

	traces, metrics := collector.payloads()
	if len(traces) == 0 {
		t.Fatal("no trace payload was exported")
	}
	if len(metrics) == 0 {
		t.Fatal("no metric payload was exported")
	}
	secrets := []string{
		"matched-SECRET-token",
		"redirect-SECRET-token",
		"mux-SECRET-token",
		"missing-SECRET-token",
		"method-SECRET-token",
		"host-SECRET-token",
		"panic-SECRET-token",
		"reject-SECRET-token",
	}
	for _, payload := range traces {
		assertPayloadOmits(t, "trace", payload, secrets...)
	}
	for _, payload := range metrics {
		assertPayloadOmits(t, "metric", payload, secrets...)
	}

	paths := make(map[string]int)
	for _, span := range decodeSpans(t, traces) {
		paths[span.attributes["url.path"]]++
	}
	for _, route := range []string{
		"/preview/{token}/notes",
		"/preview/{token}/go",
		"/preview/{token}/panic",
	} {
		if paths[route] == 0 {
			t.Errorf("no span recorded url.path=%q; paths: %v", route, paths)
		}
	}
	if paths["/[redacted]"] < 3 {
		t.Errorf("redacted paths = %d, want at least 3 for unmatched or rejected requests; paths: %v",
			paths["/[redacted]"], paths)
	}
}

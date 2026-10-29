package inertia

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// healthServer serves a switchable SSR /health endpoint so a test can model a
// sidecar that becomes unhealthy and then recovers.
func healthServer(t *testing.T, healthy *atomic.Bool) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/health" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if !healthy.Load() {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"status":"degraded"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}))
}

func readinessRenderer(t *testing.T, ssrURL string, options ...Option) *Renderer {
	t.Helper()

	ssrClient, err := NewHTTPRenderer(ssrURL, 250*time.Millisecond, 2<<20)
	if err != nil {
		t.Fatalf("NewHTTPRenderer: %v", err)
	}

	var captured RootData
	base := []Option{WithRoot(testRoot(&captured)), WithSSRRenderer(ssrClient)}
	renderer, err := newTestRenderer(append(base, options...)...)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	return renderer
}

func TestReadinessIsDisabledWithoutRendererSSRDefault(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := healthServer(t, &healthy)
	defer server.Close()

	readiness := readinessRenderer(t, server.URL).Readiness(context.Background())
	if readiness.Status != SSRStatusDisabled || !readiness.Ready() {
		t.Fatalf("readiness = %#v, want disabled and ready", readiness)
	}
}

func TestReadinessReportsExplicitCSRFallbackPolicy(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(false)
	server := healthServer(t, &healthy)
	defer server.Close()

	renderer := readinessRenderer(t, server.URL, WithSSRDefault(true), WithSSRFailFast(false))
	readiness := renderer.Readiness(context.Background())
	if readiness.Status != SSRStatusCSRFallback || !readiness.Ready() {
		t.Fatalf("readiness = %#v, want csr_fallback and ready", readiness)
	}
}

func TestReadinessProbesSidecarAndRecovers(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := healthServer(t, &healthy)
	defer server.Close()

	renderer := readinessRenderer(t, server.URL, WithSSRDefault(true), WithSSRFailFast(true))

	if readiness := renderer.Readiness(context.Background()); readiness.Status != SSRStatusReady {
		t.Fatalf("healthy readiness = %#v, want ready", readiness)
	}

	healthy.Store(false)
	readiness := renderer.Readiness(context.Background())
	if readiness.Status != SSRStatusUnavailable || readiness.Ready() {
		t.Fatalf("unhealthy readiness = %#v, want unavailable and not ready", readiness)
	}
	if readiness.Err == nil {
		t.Fatal("unhealthy readiness lost the probe failure cause")
	}

	healthy.Store(true)
	if readiness := renderer.Readiness(context.Background()); readiness.Status != SSRStatusReady {
		t.Fatalf("recovered readiness = %#v, want ready after the sidecar recovered", readiness)
	}
}

func TestReadinessIsUnavailableWhenSidecarIsStopped(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := healthServer(t, &healthy)

	renderer := readinessRenderer(t, server.URL, WithSSRDefault(true), WithSSRFailFast(true))
	if readiness := renderer.Readiness(context.Background()); readiness.Status != SSRStatusReady {
		t.Fatalf("initial readiness = %#v, want ready", readiness)
	}

	server.Close()

	readiness := renderer.Readiness(context.Background())
	if readiness.Status != SSRStatusUnavailable || readiness.Ready() {
		t.Fatalf("stopped sidecar readiness = %#v, want unavailable", readiness)
	}
}

func TestReadinessBoundsTheHealthProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	ssrClient, err := NewHTTPRenderer(server.URL, 25*time.Millisecond, 2<<20)
	if err != nil {
		t.Fatalf("NewHTTPRenderer: %v", err)
	}
	var captured RootData
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssrClient),
		WithSSRDefault(true),
		WithSSRFailFast(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	started := time.Now()
	readiness := renderer.Readiness(context.Background())
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("readiness probe was not bounded: %s", elapsed)
	}
	if readiness.Status != SSRStatusUnavailable {
		t.Fatalf("readiness = %#v, want unavailable for a timed-out health probe", readiness)
	}
}

func TestReadinessPropagatesCancellation(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := healthServer(t, &healthy)
	defer server.Close()

	renderer := readinessRenderer(t, server.URL, WithSSRDefault(true), WithSSRFailFast(true))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	readiness := renderer.Readiness(ctx)
	if readiness.Status != SSRStatusUnavailable {
		t.Fatalf("canceled readiness = %#v, want unavailable", readiness)
	}
	if !errors.Is(readiness.Err, context.Canceled) {
		t.Fatalf("readiness cause = %v, want the canceled context", readiness.Err)
	}
}

func TestReadinessIsUnsupportedForDevelopmentViteTransport(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithEnvironment("development"),
		WithSSRDefault(true),
		WithSSRFailFast(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	readiness := renderer.Readiness(context.Background())
	if readiness.Status != SSRStatusUnsupported || !readiness.Ready() {
		t.Fatalf("readiness = %#v, want unsupported and ready for the Vite transport", readiness)
	}
}

func TestRendererHealthAdapterIsSecretFree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	renderer := readinessRenderer(t, server.URL, WithSSRDefault(true), WithSSRFailFast(true))

	err := renderer.Health(context.Background())
	if err == nil {
		t.Fatal("Health returned nil for an unavailable sidecar")
	}
	var readinessErr *ReadinessError
	if !errors.As(err, &readinessErr) || readinessErr.Status != SSRStatusUnavailable {
		t.Fatalf("Health error = %#v, want a ReadinessError with unavailable status", err)
	}
	if strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("readiness error leaks the transport URL: %q", err)
	}
	if readinessErr.Err == nil {
		t.Fatal("ReadinessError lost the probe cause")
	}

	var healthy atomic.Bool
	healthy.Store(true)
	healthyServer := healthServer(t, &healthy)
	defer healthyServer.Close()
	if err := readinessRenderer(
		t,
		healthyServer.URL,
		WithSSRDefault(true),
		WithSSRFailFast(true),
	).Health(context.Background()); err != nil {
		t.Fatalf("Health for a healthy sidecar = %v, want nil", err)
	}
}

func TestSSRRuntimeHealthProbesTheListenEndpoint(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := healthServer(t, &healthy)
	defer server.Close()

	runtime, err := NewSSRRuntime(SSRRuntimeConfig{
		Executable:     "node",
		BundlePath:     "assets/dist/ssr/ssr.js",
		ListenURL:      server.URL,
		StartupTimeout: time.Second,
		MinimumMajor:   22,
		HealthTimeout:  250 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewSSRRuntime: %v", err)
	}

	if err := runtime.Health(context.Background()); err != nil {
		t.Fatalf("runtime Health = %v, want nil", err)
	}

	healthy.Store(false)
	if err := runtime.Health(context.Background()); err == nil {
		t.Fatal("runtime Health = nil for an unhealthy sidecar")
	}
}

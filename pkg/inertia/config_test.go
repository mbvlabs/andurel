package inertia

import (
	"testing"
	"time"
)

func TestNewRendererWiresHTTPClientOnly(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if renderer.ssr == nil {
		t.Fatal("expected HTTP SSR client for WithSSR pages")
	}
	if renderer.ssrURL != "http://127.0.0.1:13714" {
		t.Fatalf("ssrURL = %q", renderer.ssrURL)
	}
}

func TestNewRendererAllowsServiceHostname(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(
		"app",
		"/assets/dist/vite/*",
		"resources/js/app.ts",
		"http://localhost:5173/assets/dist",
		"http://ssr-service:13714",
		2*time.Second,
		2<<20,
		WithRoot(testRoot(nil)),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	if renderer.ssrURL != "http://ssr-service:13714" {
		t.Fatalf("ssrURL = %q", renderer.ssrURL)
	}
}

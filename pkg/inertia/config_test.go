package inertia

import "testing"

func TestNewRendererWiresHTTPClientOnly(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(
		WithContainerID("app"),
		WithRoot(testRoot(nil)),
		WithSSRURL("http://127.0.0.1:13714"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if renderer.ssr == nil {
		t.Fatal("expected HTTP SSR client for WithSSR pages")
	}
	if renderer.ssrConfig.URL != "http://127.0.0.1:13714" {
		t.Fatalf("ssrConfig.URL = %q", renderer.ssrConfig.URL)
	}
}

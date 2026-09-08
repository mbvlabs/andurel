package inertia

import "testing"

func TestNewRendererWiresHTTPClientOnly(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(nil)))
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

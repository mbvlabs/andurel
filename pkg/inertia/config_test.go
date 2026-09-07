package inertia

import "testing"

func TestConfigureSSRUsesHTTPClientOnly(t *testing.T) {
	t.Parallel()

	renderer, err := New(
		WithContainerID("app"),
		WithRoot(testRoot(nil)),
		WithSSRURL("http://127.0.0.1:13714"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if renderer.runtime != nil {
		t.Fatal("cmd/app renderer must not own a managed SSR runtime")
	}
	if renderer.ssr == nil {
		t.Fatal("expected HTTP SSR client for WithSSR pages")
	}
}

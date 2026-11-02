package inertia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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

func TestDefaultAssetVersion(t *testing.T) {
	cfg := RendererConfig{ContainerID: "app", EntryPoint: "app.ts", BuildPathURL: "/assets/*", SSRURL: "http://localhost:13714", SSRTimeout: time.Second, SSRMaxResponseBytes: 1 << 20}
	makeRenderer := func(manifest string, options ...Option) *Renderer {
		t.Helper()
		base := []Option{WithRoot(testRoot(nil)), WithEnvironment("production"), WithAssetFS(fstest.MapFS{
			"dist/vite/manifest.json": &fstest.MapFile{Data: []byte(manifest)},
		})}
		r, err := NewRenderer(cfg, append(base, options...)...)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	a := `{"app.ts":{"file":"a.js"}}`
	b := `{"app.ts":{"file":"b.js"}}`
	first, same, changed := makeRenderer(a), makeRenderer(a), makeRenderer(b)
	if len(first.version) != 64 || first.version != same.version || first.version == changed.version {
		t.Fatalf("versions not stable and content-derived: %q %q %q", first.version, same.version, changed.version)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
	req.Header.Set(HeaderInertia, "true")
	req.Header.Set(HeaderVersion, first.version)
	rec := httptest.NewRecorder()
	called := false
	handler := changed.Middleware()(func(context.Context) error { called = true; return nil })
	if err := handler(testContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if called || rec.Code != http.StatusConflict || rec.Header().Get(HeaderLocation) != "/page" || rec.Header().Get(HeaderVersion) != changed.version {
		t.Fatalf("changed manifest: called=%v status=%d headers=%v", called, rec.Code, rec.Header())
	}
	if got := makeRenderer(a, WithVersion("")).version; got != "" {
		t.Fatalf("explicit empty version = %q", got)
	}
	if got := makeRenderer(a, WithVersion("custom")).version; got != "custom" {
		t.Fatalf("explicit version = %q", got)
	}
	dev, err := NewRenderer(cfg, WithRoot(testRoot(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if dev.version == "" || strings.Contains(dev.version, "*") {
		t.Fatalf("development version = %q", dev.version)
	}
}

func TestNewRendererAllowsServiceHostname(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(
		RendererConfig{
			ContainerID:         "app",
			BuildPathURL:        "/assets/dist/vite/*",
			EntryPoint:          "resources/js/app.ts",
			ViteDevURL:          "http://localhost:5173/assets/dist",
			SSRURL:              "http://ssr-service:13714",
			SSRTimeout:          2 * time.Second,
			SSRMaxResponseBytes: 2 << 20,
		},
		WithRoot(testRoot(nil)),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	if renderer.ssrURL != "http://ssr-service:13714" {
		t.Fatalf("ssrURL = %q", renderer.ssrURL)
	}
}

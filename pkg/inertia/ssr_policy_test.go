package inertia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestSSRDefaultRendersInitialDocumentWithoutPageOptIn(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">ssr</div>`,
	}}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
		WithSSRDefault(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Welcome", nil).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if ssr.calls != 1 || captured.SSR == nil {
		t.Fatalf("SSR calls = %d, captured = %#v", ssr.calls, captured.SSR)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "ssr") {
		t.Fatalf("body = %q, want server-rendered document", body)
	}
}

func TestSSRDefaultKeepsInertiaJSONVisitsClientSide(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">ssr</div>`,
	}}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
		WithSSRDefault(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Welcome", nil).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if ssr.calls != 0 {
		t.Fatalf("SSR renderer contacted for an Inertia JSON visit: %d calls", ssr.calls)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"component":"Welcome"`) {
		t.Fatalf("body = %q, want the page object", body)
	}
}

func TestPageCSROptsOutOfRendererSSRDefault(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">ssr</div>`,
	}}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
		WithSSRDefault(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Dashboard", nil).CSR().Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if ssr.calls != 0 || captured.SSR != nil {
		t.Fatalf("CSR page rendered through SSR: calls=%d captured=%#v", ssr.calls, captured.SSR)
	}
	if body := recorder.Body.String(); body != "client" {
		t.Fatalf("body = %q, want the client-rendered document", body)
	}
}

func TestPageSSRWinsOverCSR(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">ssr</div>`,
	}}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
		WithSSRDefault(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	page := renderer.Page(testContext(request, recorder), "Dashboard", nil).CSR().SSR()
	if err := page.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if ssr.calls != 1 || captured.SSR == nil {
		t.Fatalf("explicit SSR did not win over CSR: calls=%d captured=%#v", ssr.calls, captured.SSR)
	}
}

func TestSSRDefaultOffKeepsPageOptIn(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">ssr</div>`,
	}}
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)), WithSSRRenderer(ssr))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Welcome", nil).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if ssr.calls != 0 {
		t.Fatalf("library default SSR'd a page without opt-in: %d calls", ssr.calls)
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	recorder = httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Welcome", nil).SSR().Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if ssr.calls != 1 {
		t.Fatalf("PageBuilder.SSR() did not opt in: %d calls", ssr.calls)
	}
}

func TestSSRDefaultUsesTheEnvironmentTransport(t *testing.T) {
	t.Parallel()

	const ssrBody = `<div data-server-rendered="true" data-page="app">ssr</div>`
	paths := make(chan string, 2)
	ssrServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths <- request.URL.Path
		writer.Header().Set("Content-Type", "application/json")
		payload, err := json.Marshal(SSRResponse{Body: ssrBody})
		if err != nil {
			t.Errorf("marshal SSR response: %v", err)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer ssrServer.Close()

	var developmentRoot RootData
	development, err := NewRenderer(
		RendererConfig{
			ContainerID:         "app",
			BuildPathURL:        "/assets/dist/vite/*",
			EntryPoint:          "resources/js/app.ts",
			ViteDevURL:          ssrServer.URL + "/assets/dist",
			SSRURL:              "http://127.0.0.1:13714",
			SSRTimeout:          2 * time.Second,
			SSRMaxResponseBytes: 2 << 20,
		},
		WithRoot(testRoot(&developmentRoot)),
		WithEnvironment("development"),
		WithSSRDefault(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer(development): %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	if err := development.Page(testContext(request, recorder), "Welcome", nil).Render(); err != nil {
		t.Fatalf("development Render: %v", err)
	}
	if got := <-paths; got != viteSSREndpoint {
		t.Fatalf("development SSR path = %q, want %q", got, viteSSREndpoint)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "ssr") {
		t.Fatalf("development body = %q, want the Vite-rendered document", body)
	}

	var productionRoot RootData
	production, err := NewRenderer(
		RendererConfig{
			ContainerID:         "app",
			BuildPathURL:        "/assets/dist/vite/*",
			EntryPoint:          "resources/js/app.ts",
			ViteDevURL:          "http://localhost:5173/assets/dist",
			SSRURL:              ssrServer.URL,
			SSRTimeout:          2 * time.Second,
			SSRMaxResponseBytes: 2 << 20,
		},
		WithRoot(testRoot(&productionRoot)),
		WithAssetFS(fstest.MapFS{
			"dist/vite/manifest.json": &fstest.MapFile{Data: []byte(
				`{"resources/js/app.ts":{"file":"app-abc.js","css":[]}}`,
			)},
		}),
		WithEnvironment("production"),
		WithSSRDefault(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer(production): %v", err)
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	recorder = httptest.NewRecorder()
	if err := production.Page(testContext(request, recorder), "Welcome", nil).Render(); err != nil {
		t.Fatalf("production Render: %v", err)
	}
	if got := <-paths; got != "/render" {
		t.Fatalf("production SSR path = %q, want /render", got)
	}
}

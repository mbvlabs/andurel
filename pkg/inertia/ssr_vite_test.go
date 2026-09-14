package inertia

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestViteSSRURLStripsAssetBase(t *testing.T) {
	t.Parallel()

	got, err := viteSSRURL("http://localhost:5173/assets/dist")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "http://localhost:5173/__inertia_ssr" {
		t.Fatalf("vite SSR URL = %q", got.String())
	}
}

func TestViteSSRRendererPostsToInertiaEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		gotBody, _ = io.ReadAll(request.Body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(
			`{"head":["<title>SSR</title>"],"body":"<div data-server-rendered=\"true\" data-page=\"{}\"></div>"}`,
		))
	}))
	t.Cleanup(server.Close)

	renderer, err := NewViteSSRRenderer(server.URL+"/assets/dist", 2*time.Second, 2<<20)
	if err != nil {
		t.Fatalf("NewViteSSRRenderer: %v", err)
	}

	response, err := renderer.Render(context.Background(), Page{Component: "Documentation/Show"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if gotPath != viteSSREndpoint {
		t.Fatalf("path = %q, want %s", gotPath, viteSSREndpoint)
	}
	var page Page
	if err := json.Unmarshal(gotBody, &page); err != nil {
		t.Fatalf("request body: %v", err)
	}
	if page.Component != "Documentation/Show" {
		t.Fatalf("component = %q", page.Component)
	}
	if response == nil || response.Body == "" || len(response.Head) != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestViteSSRRendererRejectsNullDocument(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("null"))
	}))
	t.Cleanup(server.Close)

	renderer, err := NewViteSSRRenderer(server.URL, 2*time.Second, 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, err = renderer.Render(context.Background(), Page{Component: "Home"})
	if err == nil {
		t.Fatal("expected error for null Vite SSR response")
	}
}

func TestNewRendererUsesViteSSRInDevelopment(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(
			`{"head":[],"body":"<div data-server-rendered=\"true\" data-page=\"{}\"></div>"}`,
		))
	}))
	t.Cleanup(server.Close)

	renderer, err := NewRenderer(
		"app",
		"/assets/dist/vite/*",
		"resources/js/app.tsx",
		server.URL+"/assets/dist",
		"http://127.0.0.1:13714",
		2*time.Second,
		2<<20,
		WithRoot(testRoot(nil)),
		WithEnvironment("development"),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	vite, ok := renderer.ssr.(*ViteSSRRenderer)
	if !ok {
		t.Fatalf("ssr type = %T, want *ViteSSRRenderer", renderer.ssr)
	}

	if _, err := vite.Render(context.Background(), Page{Component: "Home"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if gotPath != viteSSREndpoint {
		t.Fatalf("path = %q, want %s", gotPath, viteSSREndpoint)
	}
}

func TestNewRendererKeepsHTTPRendererOutsideDevelopment(t *testing.T) {
	t.Parallel()

	renderer, err := newTestRenderer(WithRoot(testRoot(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := renderer.ssr.(*HTTPRenderer); !ok {
		t.Fatalf("ssr type = %T, want *HTTPRenderer", renderer.ssr)
	}
}

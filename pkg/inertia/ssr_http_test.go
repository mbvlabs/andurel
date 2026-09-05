package inertia

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPRendererRenderAndHealth(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			switch request.URL.Path {
			case "/health":
				_, _ = writer.Write([]byte(`{"status":"ok"}`))
			case "/render":
				_, _ = writer.Write(
					[]byte(
						`{"head":["<title>SSR</title>"],"body":"<div data-server-rendered=\"true\" data-page=\"app\"></div>"}`,
					),
				)
			default:
				http.NotFound(writer, request)
			}
		}),
	)
	defer server.Close()

	config := SSRConfig{
		URL:              server.URL,
		Timeout:          2 * time.Second,
		MaxResponseBytes: 2 << 20,
	}
	renderer, err := NewHTTPRenderer(config)
	if err != nil {
		t.Fatalf("NewHTTPRenderer: %v", err)
	}
	if err := renderer.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	response, err := renderer.Render(context.Background(), Page{Component: "Home"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if response == nil || len(response.Head) != 1 || response.Body == "" {
		t.Fatalf("response = %#v", response)
	}
}

func TestHTTPRendererBoundsResponseAndTimeout(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/slow" {
				time.Sleep(50 * time.Millisecond)
			}
			_, _ = writer.Write(make([]byte, 32))
		}),
	)
	defer server.Close()

	config := SSRConfig{
		URL:              server.URL,
		Timeout:          2 * time.Second,
		MaxResponseBytes: 8,
	}
	renderer, err := NewHTTPRenderer(config)
	if err != nil {
		t.Fatalf("NewHTTPRenderer: %v", err)
	}
	_, err = renderer.Render(context.Background(), Page{})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversized response error = %v", err)
	}

	config.Timeout = time.Millisecond
	renderer, err = NewHTTPRenderer(config)
	if err != nil {
		t.Fatalf("NewHTTPRenderer timeout: %v", err)
	}
	request, cancel, err := renderer.request(context.Background(), http.MethodGet, "/slow", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer cancel()
	_, err = renderer.client.Do(request)
	if err == nil {
		t.Fatal("timeout request unexpectedly succeeded")
	}
}

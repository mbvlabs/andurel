package anduin

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// compressibleBody is larger than minCompressSize and compresses well.
func compressibleBody() string {
	return strings.Repeat("<p>documentation page</p>", 128)
}

func gzipHandler(contentType, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = io.WriteString(w, body)
	})
}

func serveCompressed(t *testing.T, handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	Compress(handler).ServeHTTP(recorder, request)

	return recorder
}

func decompress(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	reader, err := gzip.NewReader(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}

	return string(body)
}

func TestCompressGzipsLargeCompressibleBodies(t *testing.T) {
	body := compressibleBody()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "1")
		_, _ = io.WriteString(w, body)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip, deflate, br")

	recorder := serveCompressed(t, handler, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := recorder.Header().Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length = %q, want empty for a compressed body", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want %q", got, "Accept-Encoding")
	}
	if got := decompress(t, recorder); got != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestCompressLeavesSmallBodiesAlone(t *testing.T) {
	body := "<p>short page</p>"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "17")
		_, _ = io.WriteString(w, body)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, handler, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := recorder.Header().Get("Content-Length"); got != "17" {
		t.Fatalf("Content-Length = %q, want 17", got)
	}
	if got := recorder.Body.String(); got != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestCompressHonoursAcceptEncoding(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
		compress       bool
	}{
		{name: "absent header", acceptEncoding: "", compress: false},
		{name: "gzip", acceptEncoding: "gzip", compress: true},
		{name: "x-gzip", acceptEncoding: "x-gzip", compress: true},
		{name: "gzip with quality", acceptEncoding: "gzip;q=0.5", compress: true},
		{name: "gzip refused", acceptEncoding: "gzip;q=0", compress: false},
		{name: "wildcard", acceptEncoding: "*", compress: true},
		{name: "wildcard refused", acceptEncoding: "*;q=0", compress: false},
		{name: "identity only", acceptEncoding: "identity", compress: false},
		{name: "brotli only", acceptEncoding: "br", compress: false},
		{name: "gzip listed after brotli", acceptEncoding: "br, gzip", compress: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.acceptEncoding != "" {
				request.Header.Set("Accept-Encoding", test.acceptEncoding)
			}

			recorder := serveCompressed(t, gzipHandler("text/plain", compressibleBody()), request)
			got := recorder.Header().Get("Content-Encoding") == "gzip"
			if got != test.compress {
				t.Fatalf("compressed = %v, want %v", got, test.compress)
			}
		})
	}
}

func TestCompressSkipsNonCompressibleContentTypes(t *testing.T) {
	tests := []string{
		"image/png",
		"image/jpeg",
		"font/woff2",
		"audio/mpeg",
		"video/mp4",
		"application/zip",
		"application/pdf",
		"application/wasm",
		"application/octet-stream",
	}

	for _, contentType := range tests {
		t.Run(contentType, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Accept-Encoding", "gzip")
			recorder := serveCompressed(t, gzipHandler(contentType, compressibleBody()), request)

			if got := recorder.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q for %q, want empty", got, contentType)
			}
			if got := recorder.Body.String(); got != compressibleBody() {
				t.Fatalf("body was modified for %q", contentType)
			}
		})
	}
}

func TestCompressGzipsSVG(t *testing.T) {
	body := strings.Repeat("<svg><text>chart</text></svg>", 128)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, gzipHandler("image/svg+xml", body), request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := decompress(t, recorder); got != body {
		t.Fatalf("body round trip failed")
	}
}

func TestCompressSniffsMissingContentType(t *testing.T) {
	body := compressibleBody()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, gzipHandler("", body), request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := decompress(t, recorder); got != body {
		t.Fatalf("body round trip failed")
	}
}

func TestCompressSkipsAlreadyEncodedResponses(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "br")
		_, _ = io.WriteString(w, compressibleBody())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, handler, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if got := recorder.Body.String(); !strings.Contains(got, "documentation page") {
		t.Fatalf("body = %q, want the unchanged response", got)
	}
}

func TestCompressSkipsHeadRequests(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodHead, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, handler, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want %q", got, "Accept-Encoding")
	}
}

func TestCompressSkipsUpgrades(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "upgrade header", headers: map[string]string{"Upgrade": "websocket"}},
		{name: "connection header", headers: map[string]string{"Connection": "Upgrade"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusSwitchingProtocols)
				_, _ = io.WriteString(w, compressibleBody())
			})

			request := httptest.NewRequest(http.MethodGet, "/socket", nil)
			request.Header.Set("Accept-Encoding", "gzip")
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}

			recorder := serveCompressed(t, handler, request)
			if got := recorder.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want empty", got)
			}
			if got := recorder.Body.String(); got != compressibleBody() {
				t.Fatalf("body was modified for an upgrade request")
			}
		})
	}
}

func TestCompressSkipsEventStreams(t *testing.T) {
	tests := []struct {
		name         string
		accept       string
		contentType  string
		wantEncoding string
	}{
		{
			name:        "event stream content type",
			accept:      "gzip",
			contentType: "text/event-stream",
		},
		{
			name:        "event stream accept header",
			accept:      "gzip, text/event-stream",
			contentType: "text/plain",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := "data: " + compressibleBody() + "\n\n"
			request := httptest.NewRequest(http.MethodGet, "/events", nil)
			request.Header.Set("Accept-Encoding", test.accept)
			request.Header.Set("Accept", test.accept)

			recorder := serveCompressed(t, gzipHandler(test.contentType, body), request)
			if got := recorder.Header().Get("Content-Encoding"); got != test.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, test.wantEncoding)
			}
			if got := recorder.Body.String(); got != body {
				t.Fatalf("body = %q, want %q", got, body)
			}
		})
	}
}

func TestCompressLeavesBodylessResponsesAlone(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(status)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Accept-Encoding", "gzip")
			recorder := serveCompressed(t, handler, request)

			if recorder.Code != status {
				t.Fatalf("status = %d, want %d", recorder.Code, status)
			}
			if got := recorder.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want empty", got)
			}
			if recorder.Body.Len() != 0 {
				t.Fatalf("body = %q, want empty", recorder.Body.String())
			}
		})
	}
}

func TestCompressFlushesBufferedBodies(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, compressibleBody())

		controller := http.NewResponseController(w)
		if err := controller.Flush(); err != nil {
			t.Errorf("Flush: %v", err)
		}

		_, _ = io.WriteString(w, "<p>after flush</p>")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, handler, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip after a flush", got)
	}
	if got := decompress(t, recorder); !strings.Contains(got, "after flush") {
		t.Fatalf("body = %q, want the chunk written after the flush", got)
	}
}

func TestCompressDoesNotDuplicateVary(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, compressibleBody())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, handler, request)

	values := recorder.Header().Values("Vary")
	if len(values) != 1 || values[0] != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want a single Accept-Encoding value", values)
	}
}

func TestCompressPreservesStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, compressibleBody())
	})

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := serveCompressed(t, handler, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
}

func TestCompressOverHTTP(t *testing.T) {
	body := compressibleBody()
	server := httptest.NewServer(Compress(gzipHandler("text/html; charset=utf-8", body)))
	defer server.Close()

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Accept-Encoding", "gzip")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}

	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if string(decoded) != body {
		t.Fatalf("body round trip failed over HTTP")
	}
}

func TestCompressCoversErrorHandlerPages(t *testing.T) {
	errorPage := strings.Repeat("<h1>Internal Server Error</h1>", 128)

	router := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		return HTML(ctx, StatusCode(err), errorPage)
	}))
	router.Get("/boom", func(context.Context) error {
		return errors.New("boom")
	})

	server := httptest.NewServer(Compress(router.Handler()))
	defer server.Close()

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	request, err := http.NewRequest(http.MethodGet, server.URL+"/boom", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Accept-Encoding", "gzip")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.StatusCode)
	}
	if got := response.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip for an ErrorHandler page", got)
	}

	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if string(decoded) != errorPage {
		t.Fatalf("error page round trip failed")
	}
}

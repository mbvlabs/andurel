package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestViteWebSocketProxy(t *testing.T) {
	upgrader := websocket.Upgrader{}
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/assets/dist/?token=abc" {
			t.Errorf("URI %s", r.URL.RequestURI())
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = c.Close() }() // Best-effort connection cleanup.
		_ = c.WriteMessage(websocket.TextMessage, []byte("connected"))
	}))
	defer vite.Close()
	ps, err := NewServer(vite.URL, "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.ConfigureVite(vite.URL + "/assets/dist"); err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(ps.Handler(http.NotFoundHandler()))
	defer front.Close()
	header := http.Header{"Origin": {front.URL}}
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(front.URL, "http")+"/assets/dist/?token=abc", header)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }() // Best-effort connection cleanup.
	_, msg, err := c.ReadMessage()
	if err != nil || string(msg) != "connected" {
		t.Fatalf("message %q error %v", msg, err)
	}
}

func TestAllowedViteHost(t *testing.T) {
	hosts := []string{"app.localtest.me:3000", "admin.localtest.me:3000"}
	for _, tc := range []struct {
		host string
		want bool
	}{{"admin.localtest.me:3000", true}, {"shop.app.localtest.me:3000", true}, {"evil.test:3000", false}, {"app.localtest.me:1234", false}} {
		if got := allowedViteHost(tc.host, hosts); got != tc.want {
			t.Errorf("%s: %v", tc.host, got)
		}
	}
}

func TestViteProxyRoutesAssetsAndRejectsCrossOrigin(t *testing.T) {
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host+" "+r.URL.RequestURI()+" "+r.Header.Get("Origin"))
	}))
	defer vite.Close()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "app "+r.Host) }))
	defer app.Close()
	ps, err := NewServer(app.URL, "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.ConfigureVite(vite.URL + "/assets/dist"); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"app.example.test:3000", "admin.example.test:3000"} {
		r := httptest.NewRequest("GET", "http://"+host+"/assets/dist/node_modules/font.woff2?v=1", nil)
		r.Header.Set("Origin", "http://"+host)
		w := httptest.NewRecorder()
		ps.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
		want := strings.TrimPrefix(vite.URL, "http://") + " /assets/dist/node_modules/font.woff2?v=1 " + vite.URL
		if w.Code != 200 || w.Body.String() != want {
			t.Fatalf("got %d %q want %q", w.Code, w.Body.String(), want)
		}
	}
	r := httptest.NewRequest("GET", "http://admin.example.test:3000/assets/dist/@vite/client", nil)
	r.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	ps.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross origin status %d", w.Code)
	}
	w = httptest.NewRecorder()
	ps.ServeHTTP(w, httptest.NewRequest("GET", "http://admin.example.test:3000/assets/distraction", nil))
	if w.Body.String() != "app admin.example.test:3000" {
		t.Fatal(w.Body.String())
	}
}

func TestViteHTMLUsesSameOrigin(t *testing.T) {
	ps := &Server{viteURL: "http://localhost:5173/assets/dist", viteProxy: &httputil.ReverseProxy{}}
	body := `<html><head><script src="http://localhost:5173/assets/dist/@vite/client"></script><link rel="stylesheet" href="/assets/dist/style.css"></head></html>`
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}
	if err := ps.modifyResponse(resp); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(got), "localhost:5173") || strings.Contains(string(got), "/__andurel/assets/dist/") {
		t.Fatal(string(got))
	}
}

func TestViteProxyReplacesUntrustedForwardedHeaders(t *testing.T) {
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Forwarded-For"); strings.Contains(got, "203.0.113.1") {
			t.Errorf("untrusted forwarded address reached Vite: %q", got)
		}
		if got := r.Header.Get("X-Forwarded-Host"); got != "app.example.test:3000" {
			t.Errorf("forwarded host = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("authorization = %q", got)
		}
	}))
	defer vite.Close()
	ps, err := NewServer(vite.URL, "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.ConfigureVite(vite.URL + "/assets/dist"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://app.example.test:3000/assets/dist/@vite/client", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	ps.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestProxyPreservesBrowserHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host)
	}))
	defer upstream.Close()
	ps, err := NewServer(upstream.URL, "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://admin.example.test:3000/secure", nil)
	rec := httptest.NewRecorder()
	ps.ServeHTTP(rec, req)
	if got := rec.Body.String(); got != "admin.example.test:3000" {
		t.Fatalf("upstream Host = %q", got)
	}
}

type failingCloseBody struct{ io.Reader }

func (f failingCloseBody) Close() error { return errors.New("body close failed") }

func TestModifyResponseRejectsBodyCloseFailure(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Type": {"text/html"}},
		Body:   failingCloseBody{strings.NewReader("<html></html>")},
	}
	if err := (&Server{}).modifyResponse(resp); err == nil || !strings.Contains(err.Error(), "body close failed") {
		t.Fatalf("expected body close failure, got %v", err)
	}
}

func TestModifyResponseRejectsInvalidCompression(t *testing.T) {
	for _, encoding := range []string{"gzip", "br"} {
		t.Run(encoding, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Type": {"text/html"}, "Content-Encoding": {encoding}},
				Body:   io.NopCloser(strings.NewReader("not compressed")),
			}
			if err := (&Server{}).modifyResponse(resp); err == nil {
				t.Fatal("expected invalid compressed body to fail")
			}
		})
	}
}

func TestServeLocalAssetGET(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "assets", "css", "style.css")
	if err := os.MkdirAll(filepath.Dir(cssPath), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "body { margin-top: 1rem; }"
	if err := os.WriteFile(cssPath, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	ps := &Server{projectRoot: dir}
	req := httptest.NewRequest(http.MethodGet, "/__andurel/assets/css/style.css", nil)
	rec := httptest.NewRecorder()

	if ok := ps.serveLocalAsset(rec, req); !ok {
		t.Fatal("expected local asset to be served")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != want {
		t.Fatalf("unexpected body: got %q want %q", rec.Body.String(), want)
	}
	if got := rec.Header().Get("Content-Type"); got == "" {
		t.Fatal("expected content type header to be set")
	}
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Fatal("expected Cache-Control header to be set")
	}
}

func TestServeLocalAssetHEAD(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "assets", "css", "style.css")
	if err := os.MkdirAll(filepath.Dir(cssPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "h1 { margin: 0; }"
	if err := os.WriteFile(cssPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ps := &Server{projectRoot: dir}
	req := httptest.NewRequest(http.MethodHead, "/__andurel/assets/css/style.css", nil)
	rec := httptest.NewRecorder()

	if ok := ps.serveLocalAsset(rec, req); !ok {
		t.Fatal("expected local asset to be served")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body for HEAD request, got %d bytes", rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(content)) {
		t.Fatalf("unexpected content-length: got %q want %q", got, strconv.Itoa(len(content)))
	}
	if got := rec.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("unexpected content-type: got %q", got)
	}
}

func TestServeLocalAssetRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	ps := &Server{projectRoot: dir}
	req := httptest.NewRequest(http.MethodGet, "/__andurel/assets/../../etc/passwd", nil)
	rec := httptest.NewRecorder()

	if ok := ps.serveLocalAsset(rec, req); ok {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestServeLocalAssetTimestampedPathFallback(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "assets", "css", "style.css")
	if err := os.MkdirAll(filepath.Dir(cssPath), 0o755); err != nil {
		t.Fatal(err)
	}
	want := ".x { margin-top: 1rem; }"
	if err := os.WriteFile(cssPath, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	ps := &Server{projectRoot: dir}
	req := httptest.NewRequest(http.MethodGet, "/__andurel/assets/css/1770715671/style.css", nil)
	rec := httptest.NewRecorder()

	if ok := ps.serveLocalAsset(rec, req); !ok {
		t.Fatal("expected timestamped path to fall back to local asset")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != want {
		t.Fatalf("unexpected body: got %q want %q", rec.Body.String(), want)
	}
}

func TestModifyResponseDisablesHTMLCaching(t *testing.T) {
	ps := &Server{}
	original := "<html><head></head><body>ok</body></html>"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"text/html; charset=utf-8"},
			"Content-Length": []string{strconv.Itoa(len(original))},
		},
		Body: io.NopCloser(strings.NewReader(original)),
	}

	if err := ps.modifyResponse(resp); err != nil {
		t.Fatalf("modifyResponse returned error: %v", err)
	}

	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("expected HTML Cache-Control to disable store, got %q", got)
	}
}

func TestModifyResponseSkipsInjectionForHEAD(t *testing.T) {
	ps := &Server{}
	original := "<html><head></head><body>ok</body></html>"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"text/html; charset=utf-8"},
			"Content-Length": []string{strconv.Itoa(len(original))},
		},
		Body:    io.NopCloser(strings.NewReader(original)),
		Request: httptest.NewRequest(http.MethodHead, "http://example.com", nil),
	}

	if err := ps.modifyResponse(resp); err != nil {
		t.Fatalf("modifyResponse returned error: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}
	if string(body) != original {
		t.Fatalf("expected HEAD response to remain unchanged, got %q", string(body))
	}
	if got := resp.Header.Get("Content-Length"); got != strconv.Itoa(len(original)) {
		t.Fatalf("expected content-length unchanged, got %q", got)
	}
}

func TestModifyResponseSkipsInjectionForNoContent(t *testing.T) {
	ps := &Server{}
	original := "<html><head></head><body>ignored</body></html>"
	resp := &http.Response{
		StatusCode: http.StatusNoContent,
		Header: http.Header{
			"Content-Type":   []string{"text/html; charset=utf-8"},
			"Content-Length": []string{strconv.Itoa(len(original))},
		},
		Body: io.NopCloser(strings.NewReader(original)),
	}

	if err := ps.modifyResponse(resp); err != nil {
		t.Fatalf("modifyResponse returned error: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}
	if string(body) != original {
		t.Fatalf("expected 204 response to remain unchanged, got %q", string(body))
	}
}

func TestModifyResponseSkipsInjectionForNotModified(t *testing.T) {
	ps := &Server{}
	original := "<html><head></head><body>cached</body></html>"
	resp := &http.Response{
		StatusCode: http.StatusNotModified,
		Header: http.Header{
			"Content-Type":   []string{"text/html; charset=utf-8"},
			"Content-Length": []string{strconv.Itoa(len(original))},
		},
		Body: io.NopCloser(strings.NewReader(original)),
	}

	if err := ps.modifyResponse(resp); err != nil {
		t.Fatalf("modifyResponse returned error: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body failed: %v", err)
	}
	if string(body) != original {
		t.Fatalf("expected 304 response to remain unchanged, got %q", string(body))
	}
}

func TestProxyUnavailableReturnsAutoRetryPage(t *testing.T) {
	ps, err := NewServer("http://127.0.0.1:65535", "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	handler := ps.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "http://localhost:3000/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when upstream is unavailable, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected HTML content type, got %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "andurel run: development server restarting...") {
		t.Fatalf("expected recovery page body, got %q", body)
	}
	if !strings.Contains(body, "window.location.reload()") {
		t.Fatalf("expected auto-retry script in response body")
	}
}

func TestProxyUnavailableSkipsRetryWhenHeaderPresent(t *testing.T) {
	ps, err := NewServer("http://127.0.0.1:65535", "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3000/", nil)
	req.Header.Set(proxyRetryHeader, "1")
	rec := httptest.NewRecorder()

	start := time.Now()
	ps.handleProxyError(rec, req, errors.New("dial tcp: connection refused"))
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when upstream is unavailable, got %d", rec.Code)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected retry header to skip wait/retry path, took %s", elapsed)
	}
	if body := rec.Body.String(); !strings.Contains(body, "andurel run: development server restarting...") {
		t.Fatalf("expected fallback page body, got %q", body)
	}
}

func TestProxyUnavailableWebSocketReturnsPlainServiceUnavailable(t *testing.T) {
	ps, err := NewServer("http://127.0.0.1:65535", "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3000/__andurel/events", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	ps.handleProxyError(rec, req, errors.New("dial tcp: connection refused"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for websocket upstream failure, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("expected plain text websocket error response, got %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "upstream unavailable") {
		t.Fatalf("expected plain upstream unavailable error body, got %q", body)
	}
	if strings.Contains(rec.Body.String(), "andurel run: development server restarting...") {
		t.Fatal("did not expect HTML recovery page for websocket requests")
	}
}

func TestProxyUnavailableHeadReturnsNoBody(t *testing.T) {
	ps, err := NewServer("http://127.0.0.1:65535", "/__andurel/events", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodHead, "http://localhost:3000/", nil)
	req.Header.Set(proxyRetryHeader, "1")
	rec := httptest.NewRecorder()

	ps.handleProxyError(rec, req, errors.New("dial tcp: connection refused"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when upstream is unavailable, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body for HEAD fallback response, got %d bytes", rec.Body.Len())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected HTML content type, got %q", got)
	}
}

package anduin

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type failingFS struct {
	err error
}

func (f failingFS) Open(string) (fs.File, error) {
	return nil, f.err
}

func serveStatic(
	t *testing.T,
	handler Handler,
	pathValue string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/dist/"+pathValue, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	ctx := ReplaceRequest(context.Background(), request)
	ctx = ReplaceResponseWriter(ctx, recorder)
	SetPathValue(ctx, "path", pathValue)

	if err := handler(ctx); err != nil {
		t.Fatalf("serve %q: %v", pathValue, err)
	}

	return recorder
}

func mustStaticFS(t *testing.T, fsys fs.FS, opts ...StaticOption) Handler {
	t.Helper()

	handler, err := StaticFS(fsys, opts...)
	if err != nil {
		t.Fatalf("StaticFS: %v", err)
	}

	return handler
}

func TestStaticFSServesFilesWithTheirContentType(t *testing.T) {
	files := fstest.MapFS{
		"app.js":           {Data: []byte("console.log(1)")},
		"site.css":         {Data: []byte("body{}")},
		"data.bin":         {Data: []byte{0x01, 0x02}},
		"nested/a.andurel": {Data: []byte("svg")},
		"font.woff2":       {Data: []byte("woff2")},
		"font.woff":        {Data: []byte("woff")},
		"font.ttf":         {Data: []byte("ttf")},
		"font.otf":         {Data: []byte("otf")},
		"page.html":        {Data: []byte("<html></html>")},
		"unknown.blob":     {Data: []byte("blob")},
	}
	handler := mustStaticFS(t, files)

	tests := []struct {
		path        string
		body        string
		contentType string
	}{
		{path: "app.js", body: "console.log(1)", contentType: "application/javascript"},
		{path: "site.css", body: "body{}", contentType: "text/css"},
		{path: "data.bin", body: string([]byte{0x01, 0x02}), contentType: "application/octet-stream"},
		{path: "nested/a.andurel", body: "svg", contentType: "application/octet-stream"},
		{path: "font.woff2", body: "woff2", contentType: "font/woff2"},
		{path: "font.woff", body: "woff", contentType: "font/woff"},
		{path: "font.ttf", body: "ttf", contentType: "font/ttf"},
		{path: "font.otf", body: "otf", contentType: "font/otf"},
		{path: "page.html", body: "<html></html>", contentType: mime.TypeByExtension(".html")},
		{path: "unknown.blob", body: "blob", contentType: "application/octet-stream"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := serveStatic(t, handler, test.path, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			if recorder.Body.String() != test.body {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), test.body)
			}
			if got := recorder.Header().Get("Content-Type"); got != test.contentType {
				t.Fatalf("Content-Type = %q, want %q", got, test.contentType)
			}
		})
	}
}

func TestStaticFSCleansPathsAndAnswersMissingFilesWith404(t *testing.T) {
	files := fstest.MapFS{
		"app.js": {Data: []byte("console.log(1)")},
	}
	handler := mustStaticFS(t, files)

	cleaned := serveStatic(t, handler, "nested/../app.js", nil)
	if cleaned.Code != http.StatusOK || cleaned.Body.String() != "console.log(1)" {
		t.Fatalf("cleaned path = %d %q, want 200 %q", cleaned.Code, cleaned.Body.String(), "console.log(1)")
	}

	for _, pathValue := range []string{"", ".", "..", "../secret", "missing.txt"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/dist/"+pathValue, nil)
		ctx := ReplaceRequest(context.Background(), request)
		ctx = ReplaceResponseWriter(ctx, recorder)
		SetPathValue(ctx, "path", pathValue)

		err := handler(ctx)
		if err == nil {
			t.Fatalf("serve %q returned no error", pathValue)
		}
		if got := StatusCode(err); got != http.StatusNotFound {
			t.Fatalf("serve %q status = %d, want 404", pathValue, got)
		}
	}
}

func TestStaticFSImmutableSetsETagsAndAnswers304(t *testing.T) {
	content := []byte("console.log(1)")
	files := fstest.MapFS{
		"app.js": {Data: content},
	}
	handler := mustStaticFS(t, files, Immutable(true))

	wantETag := fmt.Sprintf(`"%x-%x"`, md5.Sum(content), len(content))

	first := serveStatic(t, handler, "app.js", nil)
	if got := first.Header().Get("ETag"); got != wantETag {
		t.Fatalf("ETag = %q, want %q", got, wantETag)
	}
	if got := first.Header().Get("Cache-Control"); got != "public, max-age=7776000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := first.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want %q", got, "Accept-Encoding")
	}

	second := serveStatic(t, handler, "app.js", map[string]string{"If-None-Match": wantETag})
	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", second.Body.String())
	}
}

func TestStaticFSWithoutImmutableSkipsCacheHeaders(t *testing.T) {
	handler := mustStaticFS(t, fstest.MapFS{
		"app.js": {Data: []byte("console.log(1)")},
	})

	recorder := serveStatic(t, handler, "app.js", nil)
	if got := recorder.Header().Get("ETag"); got != "" {
		t.Fatalf("ETag = %q, want empty", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("Cache-Control = %q, want empty", got)
	}
}

func TestServeContentSetsETagAndAnswers304(t *testing.T) {
	body := []byte("<html>generated</html>")
	wantETag := fmt.Sprintf(`"%x-%x"`, md5.Sum(body), len(body))

	request := httptest.NewRequest(http.MethodGet, "/page", nil)
	recorder := httptest.NewRecorder()
	ctx := ReplaceRequest(context.Background(), request)
	ctx = ReplaceResponseWriter(ctx, recorder)

	if err := ServeContent(ctx, "page.html", body); err != nil {
		t.Fatalf("ServeContent: %v", err)
	}

	if got := recorder.Header().Get("ETag"); got != wantETag {
		t.Fatalf("ETag = %q, want %q", got, wantETag)
	}
	if got := recorder.Header().Get("Content-Type"); got != mime.TypeByExtension(".html") {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("Cache-Control = %q, want empty for generated content", got)
	}
	if got := recorder.Body.String(); got != string(body) {
		t.Fatalf("body = %q, want %q", got, body)
	}

	notModified := httptest.NewRecorder()
	conditional := httptest.NewRequest(http.MethodGet, "/page", nil)
	conditional.Header.Set("If-None-Match", wantETag)
	ctx = ReplaceRequest(context.Background(), conditional)
	ctx = ReplaceResponseWriter(ctx, notModified)

	if err := ServeContent(ctx, "page.html", body); err != nil {
		t.Fatalf("ServeContent with If-None-Match: %v", err)
	}
	if notModified.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", notModified.Code)
	}
	if notModified.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", notModified.Body.String())
	}
}

func TestStaticFSImmutableSurfacesWalkErrors(t *testing.T) {
	_, err := StaticFS(failingFS{err: errors.New("walk failed")}, Immutable(true))
	if err == nil {
		t.Fatal("StaticFS error = nil, want walk error")
	}
	if !strings.Contains(err.Error(), "walk failed") {
		t.Fatalf("StaticFS error = %q, want walk failure", err)
	}

	if _, err := StaticFS(failingFS{err: fs.ErrNotExist}, Immutable(true)); err != nil {
		t.Fatalf("StaticFS with missing root = %v, want nil", err)
	}
}

func TestRobotsTxtMatchesScaffoldOutput(t *testing.T) {
	handler := RobotsTxt("https://example.com/sitemap.xml")

	recorder := httptest.NewRecorder()
	ctx := ReplaceResponseWriter(context.Background(), recorder)
	if err := handler(ctx); err != nil {
		t.Fatalf("RobotsTxt: %v", err)
	}

	want := "User-agent: *\nAllow: /\nSitemap: https://example.com/sitemap.xml\n"
	if recorder.Body.String() != want {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), want)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestSitemapMatchesScaffoldOutput(t *testing.T) {
	handler, err := Sitemap(SitemapURL{
		Loc:        "https://example.com",
		ChangeFreq: "monthly",
		Priority:   "1",
	})
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx := ReplaceResponseWriter(context.Background(), recorder)
	if err := handler(ctx); err != nil {
		t.Fatalf("sitemap handler: %v", err)
	}

	want := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com</loc>
    <changefreq>monthly</changefreq>
    <priority>1</priority>
  </url>
</urlset>`
	if recorder.Body.String() != want {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), want)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/xml" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestSitemapIncludesLastModOnlyWhenSet(t *testing.T) {
	lastMod := time.Date(2026, 10, 27, 12, 30, 0, 0, time.UTC)
	handler, err := Sitemap(
		SitemapURL{Loc: "https://example.com/changed", LastMod: lastMod},
		SitemapURL{Loc: "https://example.com/undated"},
	)
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx := ReplaceResponseWriter(context.Background(), recorder)
	if err := handler(ctx); err != nil {
		t.Fatalf("sitemap handler: %v", err)
	}

	want := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/changed</loc>
    <lastmod>2026-10-27T12:30:00Z</lastmod>
  </url>
  <url>
    <loc>https://example.com/undated</loc>
  </url>
</urlset>`
	if recorder.Body.String() != want {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), want)
	}
}

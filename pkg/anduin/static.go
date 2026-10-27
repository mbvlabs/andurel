package anduin

import (
	"context"
	"crypto/md5"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	immutableCacheMaxAge  = "7776000"
	immutableCacheControl = "public, max-age=" + immutableCacheMaxAge + ", immutable"
)

// staticConfig collects the options passed to StaticFS.
type staticConfig struct {
	immutable bool
}

// StaticOption configures StaticFS.
type StaticOption func(*staticConfig)

// Immutable serves files with long-lived immutable cache headers and answers a
// matching If-None-Match with 304 Not Modified. ETags are computed once, when
// StaticFS is built, because the filesystem is fixed at build time.
func Immutable(enabled bool) StaticOption {
	return func(cfg *staticConfig) {
		cfg.immutable = enabled
	}
}

// StaticFS serves files from fsys for the route wildcard "path". The caller
// roots fsys at the served directory, for example fs.Sub(assets.Files, "dist").
// With Immutable, a walk error other than a missing root is returned.
func StaticFS(fsys fs.FS, opts ...StaticOption) (Handler, error) {
	cfg := staticConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	server := staticServer{fsys: fsys}
	if cfg.immutable {
		etags, err := precomputeETags(fsys)
		if err != nil {
			return nil, err
		}
		server.etags = etags
	}

	return server.serve, nil
}

type staticServer struct {
	fsys  fs.FS
	etags map[string]string
}

func (s staticServer) serve(ctx context.Context) error {
	param := strings.TrimPrefix(PathValue(ctx, "path"), "/")
	param = path.Clean(param)
	if param == "." || param == "" || strings.HasPrefix(param, "../") {
		return NotFound(errors.New(http.StatusText(http.StatusNotFound)))
	}

	data, err := fs.ReadFile(s.fsys, param)
	if err != nil {
		return NotFound(errors.New(http.StatusText(http.StatusNotFound)))
	}

	if s.etags != nil {
		if etag, ok := s.etags[param]; ok {
			return serveContent(ctx, param, etag, data, immutableCacheControl)
		}
	}

	return serveContent(ctx, param, "", data, "")
}

func precomputeETags(fsys fs.FS) (map[string]string, error) {
	etags := make(map[string]string)
	err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		etags[name] = etagFor(content)

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return etags, nil
}

func etagFor(content []byte) string {
	//nolint:gosec //only needed for browser caching
	hash := md5.Sum(content)

	return fmt.Sprintf(`"%x-%x"`, hash, len(content))
}

// ServeContent writes body as the response for name, setting the content type
// implied by name's extension and an ETag derived from the bytes. A matching
// If-None-Match is answered with 304 Not Modified. It shares StaticFS's ETag
// format and conditional handling, so generated responses (a sitemap or a feed)
// behave like embedded assets.
func ServeContent(ctx context.Context, name string, body []byte) error {
	return serveContent(ctx, name, etagFor(body), body, "")
}

func serveContent(ctx context.Context, name, etag string, body []byte, cacheControl string) error {
	if etag != "" {
		SetHeader(ctx, "ETag", etag)
	}
	if cacheControl != "" {
		SetHeader(ctx, "Cache-Control", cacheControl)
	}
	if match := Header(ctx, "If-None-Match"); etag != "" && match == etag {
		return NoContent(ctx, http.StatusNotModified)
	}

	SetHeader(ctx, "Vary", "Accept-Encoding")

	return writeContent(ctx, http.StatusOK, contentTypeByExt(name), body)
}

func writeContent(ctx context.Context, status int, contentType string, body []byte) error {
	writer := ResponseWriter(ctx)
	if writer == nil {
		return Error(ctx, http.StatusInternalServerError, errors.New("static: no response writer on context"))
	}

	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(status)
	if len(body) == 0 {
		return nil
	}

	if _, err := writer.Write(body); err != nil {
		return Error(ctx, http.StatusInternalServerError, err)
	}

	return nil
}

// contentTypeByExt keeps the explicit table the scaffold used so JavaScript and
// fonts are typed correctly on minimal images without /etc/mime.types.
func contentTypeByExt(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript"
	case strings.HasSuffix(name, ".css"):
		return "text/css"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".woff"):
		return "font/woff"
	case strings.HasSuffix(name, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(name, ".otf"):
		return "font/otf"
	}

	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType
	}

	return "application/octet-stream"
}

// RobotsTxt serves the robots.txt body pointing at sitemapURL.
func RobotsTxt(sitemapURL string) Handler {
	body := fmt.Sprintf("User-agent: *\nAllow: /\nSitemap: %s\n", sitemapURL)

	return func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, body)
	}
}

// SitemapURL is one URL entry in a sitemap.
type SitemapURL struct {
	Loc        string
	ChangeFreq string
	Priority   string
	LastMod    time.Time
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	LastMod    string `xml:"lastmod,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URL     []sitemapURL `xml:"url"`
}

// Sitemap builds a handler serving an XML sitemap with the given URLs.
func Sitemap(urls ...SitemapURL) (Handler, error) {
	set := sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URL:   make([]sitemapURL, 0, len(urls)),
	}
	for _, url := range urls {
		set.URL = append(set.URL, sitemapURL{
			Loc:        url.Loc,
			ChangeFreq: url.ChangeFreq,
			LastMod:    w3cDate(url.LastMod),
			Priority:   url.Priority,
		})
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append([]byte(xml.Header), body...)

	return func(ctx context.Context) error {
		return writeContent(ctx, http.StatusOK, "application/xml", body)
	}, nil
}

// w3cDate formats a sitemap timestamp as a W3C datetime, or returns "" so the
// entry is omitted when no modification time is known.
func w3cDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format(time.RFC3339)
}

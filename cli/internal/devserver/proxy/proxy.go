package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/andybalholm/brotli"
)

const localAssetsPrefix = "/__andurel/assets/"
const proxyRetryHeader = "X-Andurel-Run-Upstream-Retry"

// Server is a reverse proxy that injects the hot reload script into HTML responses.
type Server struct {
	target       *url.URL
	proxy        *httputil.ReverseProxy
	wsPath       string
	projectRoot  string
	isRebuilding func() bool
	viteProxy    *httputil.ReverseProxy
	viteURL      string
	viteHosts    []string
}

// SetViteHosts limits access to the Vite proxy to configured application hosts.
// Additional single-label subdomains of the primary host follow the scaffold's
// default HOST_<NAME> convention.
func (ps *Server) SetViteHosts(hosts []string) { ps.viteHosts = hosts }

// ConfigureVite routes the generated Vite base through the browser-facing proxy.
func (ps *Server) ConfigureVite(rawURL string) error {
	target, err := url.Parse(rawURL)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.Path != "/assets/dist" {
		return fmt.Errorf("unsupported Vite development URL %q", rawURL)
	}
	upstream := &url.URL{Scheme: target.Scheme, Host: target.Host}
	p := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(upstream)
		r.SetXForwarded()
		if r.Out.Header.Get("Origin") != "" {
			r.Out.Header.Set("Origin", upstream.String())
		}
	}}
	ps.viteURL = strings.TrimRight(rawURL, "/")
	ps.viteProxy = p
	return nil
}

func (ps *Server) serveVite(w http.ResponseWriter, r *http.Request) bool {
	if ps.viteProxy == nil || (r.URL.Path != "/assets/dist" && !strings.HasPrefix(r.URL.Path, "/assets/dist/")) {
		return false
	}
	if len(ps.viteHosts) > 0 && !allowedViteHost(r.Host, ps.viteHosts) {
		http.Error(w, "unrecognized Vite host", http.StatusForbidden)
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "cross-origin Vite request denied", http.StatusForbidden)
			return true
		}
	}
	ps.viteProxy.ServeHTTP(w, r)
	return true
}

func allowedViteHost(host string, hosts []string) bool {
	for _, allowed := range hosts {
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	if len(hosts) == 0 || hosts[0] == "" {
		return false
	}
	suffix := "." + strings.ToLower(hosts[0])
	host = strings.ToLower(host)
	if !strings.HasSuffix(host, suffix) {
		return false
	}
	label := strings.TrimSuffix(host, suffix)
	return label != "" && !strings.ContainsAny(label, ".:/")
}

// NewServer creates the reload proxy in front of targetURL. projectRoot is
// where /__andurel/assets/ requests are served from (its assets/ directory).
func NewServer(targetURL, wsPath, projectRoot string, isRebuilding func() bool) (*Server, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	ps := &Server{
		target:       target,
		proxy:        proxy,
		wsPath:       wsPath,
		projectRoot:  projectRoot,
		isRebuilding: isRebuilding,
	}

	proxy.ModifyResponse = ps.modifyResponse
	proxy.ErrorHandler = ps.handleProxyError

	return ps, nil
}

func (ps *Server) modifyResponse(resp *http.Response) error {
	if isBodylessResponse(resp) {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !IsHTMLResponse(contentType) {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return err
	}

	encoding := resp.Header.Get("Content-Encoding")
	var decompressed []byte

	switch encoding {
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return err
		}
		decompressed, err = io.ReadAll(gr)
		closeErr := gr.Close()
		if err = errors.Join(err, closeErr); err != nil {
			return err
		}
	case "br":
		br := brotli.NewReader(bytes.NewReader(body))
		decompressed, err = io.ReadAll(br)
		if err != nil {
			return err
		}
	default:
		decompressed = body
	}

	if ps.viteProxy != nil {
		decompressed = bytes.ReplaceAll(decompressed, []byte(ps.viteURL+"/"), []byte("/assets/dist/"))
	}
	modified := RewriteStylesheetHrefs(decompressed)
	modified = InjectScript(modified)

	var finalBody []byte
	switch encoding {
	case "gzip":
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, writeErr := gw.Write(modified)
		closeErr := gw.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
		finalBody = buf.Bytes()
	case "br":
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		_, writeErr := bw.Write(modified)
		closeErr := bw.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
		finalBody = buf.Bytes()
	default:
		finalBody = modified
	}

	resp.Body = io.NopCloser(bytes.NewReader(finalBody))
	resp.ContentLength = int64(len(finalBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(finalBody)))
	disableDocumentCache(resp.Header)

	return nil
}

func disableDocumentCache(header http.Header) {
	header.Set("Cache-Control", "no-store, no-cache, must-revalidate")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}

func isBodylessResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	if resp.Request != nil && resp.Request.Method == http.MethodHead {
		return true
	}
	return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified
}

func (ps *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ps.serveVite(w, r) {
		return
	}
	if ps.serveLocalAsset(w, r) {
		return
	}
	ps.proxy.ServeHTTP(w, r)
}

func (ps *Server) Handler(wsHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a WebSocket request to our endpoint
		if r.URL.Path == ps.wsPath && isWebSocketRequest(r) {
			wsHandler.ServeHTTP(w, r)
			return
		}
		if ps.serveVite(w, r) {
			return
		}
		if ps.serveLocalAsset(w, r) {
			return
		}
		if ps.isRebuilding != nil && ps.isRebuilding() {
			r.Header.Set(proxyRetryHeader, "1")
			ps.handleProxyError(w, r, errors.New("server restart in progress"))
			return
		}
		ps.proxy.ServeHTTP(w, r)
	})
}

func (ps *Server) serveLocalAsset(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if !strings.HasPrefix(r.URL.Path, localAssetsPrefix) || ps.projectRoot == "" {
		return false
	}

	assetRelativePath := strings.TrimPrefix(r.URL.Path, localAssetsPrefix)
	localPath, ok := ps.resolveLocalAssetPath(assetRelativePath)
	if !ok {
		return false
	}

	if r.Method == http.MethodHead {
		fileInfo, err := os.Stat(localPath)
		if err != nil || fileInfo.IsDir() {
			return false
		}
		contentType, err := detectContentTypeFromFile(localPath)
		if err != nil {
			return false
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.WriteHeader(http.StatusOK)
		return true
	}

	content, err := os.ReadFile(localPath)
	if err != nil {
		return false
	}

	contentType := detectContentType(localPath, content)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(content)
	}
	return true
}

func detectContentType(localPath string, content []byte) string {
	contentType := mime.TypeByExtension(filepath.Ext(localPath))
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	return contentType
}

func detectContentTypeFromFile(localPath string) (string, error) {
	contentType := mime.TypeByExtension(filepath.Ext(localPath))
	if contentType != "" {
		return contentType, nil
	}

	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }() // Read-only file; there is no buffered output to flush.

	var header [512]byte
	n, readErr := file.Read(header[:])
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", readErr
	}
	return http.DetectContentType(header[:n]), nil
}

func (ps *Server) resolveLocalAssetPath(assetRelativePath string) (string, bool) {
	assetsRoot := filepath.Join(ps.projectRoot, "assets")
	candidates := []string{assetRelativePath}

	parts := strings.Split(strings.Trim(filepath.ToSlash(assetRelativePath), "/"), "/")
	if len(parts) >= 3 {
		for i := 1; i < len(parts)-1; i++ {
			if isCacheBusterSegment(parts[i]) {
				trimmed := append([]string{}, parts[:i]...)
				trimmed = append(trimmed, parts[i+1:]...)
				candidates = append(candidates, strings.Join(trimmed, "/"))
			}
		}
	}

	for _, candidate := range candidates {
		localPath := filepath.Clean(filepath.Join(assetsRoot, filepath.FromSlash(candidate)))
		rel, err := filepath.Rel(assetsRoot, localPath)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		info, err := os.Stat(localPath)
		if err == nil && !info.IsDir() {
			return localPath, true
		}
	}

	return "", false
}

func (ps *Server) handleProxyError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("[run] proxy upstream unavailable: %v", err)

	// Keep websocket handshakes as plain HTTP errors.
	if isWebSocketRequest(r) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
		return
	}

	// For idempotent navigation requests, do a short optimistic wait/retry
	// so fast app restarts avoid rendering the fallback page entirely.
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.Header.Get(proxyRetryHeader) == "" {
		if ps.waitForUpstreamReady(r.Context(), 700*time.Millisecond) {
			retryReq := r.Clone(r.Context())
			retryReq.Header = r.Header.Clone()
			retryReq.Header.Set(proxyRetryHeader, "1")
			ps.proxy.ServeHTTP(w, retryReq)
			return
		}
	}

	// For normal browser navigation, return a websocket-driven recovery page so
	// manual hard-refresh is never required during app restart windows.
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		ps.renderRestartPage(w, r)
		return
	}

	http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
}

func (ps *Server) renderRestartPage(w http.ResponseWriter, r *http.Request) {
	body := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>andurel run: development server restarting...</title>
  <style>
    body { font-family: system-ui, -apple-system, sans-serif; margin: 0; line-height: 1.45; color: #111827; display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; text-align: center; }
    code { background: #f3f4f6; padding: 0.1rem 0.35rem; border-radius: 0.25rem; }
  </style>
</head>
<body>
  <h1>andurel run: development server restarting...</h1>
  <p>andurel run is restarting development services. This page reconnects and reloads as soon as it is ready.</p>
  <p><small>Upstream: <code>%s</code></small></p>
  %s
  <script>
    setInterval(function() {
      fetch(window.location.href, { method: 'HEAD', cache: 'no-store' })
        .then(function(resp) {
          if (resp.ok) window.location.reload();
        })
        .catch(function() {});
    }, 3000);
  </script>
</body>
</html>`, ps.target.String(), HotReloadScript)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusServiceUnavailable)
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, body)
	}
}

func (ps *Server) waitForUpstreamReady(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	dialer := &net.Dialer{Timeout: 120 * time.Millisecond}
	for {
		conn, err := dialer.DialContext(ctx, "tcp", ps.target.Host)
		if err == nil {
			_ = conn.Close()
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func isCacheBusterSegment(part string) bool {
	if part == "" {
		return false
	}
	allDigits := true
	for _, r := range part {
		if !unicode.IsDigit(r) {
			allDigits = false
			break
		}
	}
	if allDigits && len(part) >= 6 {
		return true
	}
	return false
}

func isWebSocketRequest(r *http.Request) bool {
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}

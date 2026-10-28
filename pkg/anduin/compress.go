package anduin

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// minCompressSize is the smallest body worth gzipping. Below it the gzip
// header and trailer cost as much as they save, and leaving the body alone
// keeps net/http's automatic Content-Length.
const minCompressSize = 1024

// defaultCompressLevel trades a little ratio for CPU. Pages are rendered on
// every request, so the default level is a good middle ground.
const defaultCompressLevel = gzip.DefaultCompression

// gzipPool reuses encoders between requests. A gzip.Writer keeps a 40 KiB
// window, so allocating one per request would dominate the cost of compressing
// a small page.
var gzipPool = sync.Pool{
	New: func() any {
		writer, err := gzip.NewWriterLevel(io.Discard, defaultCompressLevel)
		if err != nil {
			// gzip.NewWriterLevel only fails on an invalid level, and the level
			// is a constant here.
			panic(err)
		}
		return writer
	},
}

// Compress wraps next with gzip response compression.
//
// It is an http.Handler wrapper rather than an anduin.Middleware on purpose.
// The router's ErrorHandler renders 404 and 500 pages after the middleware
// chain has returned, so a middleware-shaped compressor would close the gzip
// stream before those bytes exist and either truncate the error page or skip
// it. Wrapping router.Handler() means every response the process produces —
// pages, error pages, redirects and embedded assets — passes through one
// writer that closes the stream when the request is done. It also leaves the
// chain's Telemetry and Logger middleware measuring the handler rather than
// the compression writer.
//
// Rules:
//   - only when the request accepts gzip (a q-value of zero opts out);
//   - only for compressible content types: images, fonts, audio, video and
//     already-compressed archives pass through untouched;
//   - only for bodies of at least minCompressSize bytes;
//   - never for HEAD (net/http discards the body), 204, 304 or any other
//     bodyless response, so conditional requests keep working;
//   - never for upgrade requests or event streams, whose connections stay open
//     after the first body chunk;
//   - Content-Length is dropped when the body is compressed, because the
//     encoded length is not known yet.
//
// Every response gains Vary: Accept-Encoding so caches do not serve a gzipped
// body to a client that cannot decode it.
func Compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AddVary(w.Header(), "Accept-Encoding")

		if !compressibleRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		compressor := &compressWriter{ResponseWriter: w, request: r}
		defer compressor.close()
		next.ServeHTTP(compressor, r)
	})
}

// compressibleRequest reports whether a request may be compressed at all.
func compressibleRequest(r *http.Request) bool {
	if r.Method == http.MethodHead {
		return false
	}
	if r.Header.Get("Upgrade") != "" {
		return false
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return false
	}

	return !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

// compressWriter buffers the start of a response so it can decide whether
// compressing is worthwhile before any header is sent. Once the buffer reaches
// minCompressSize (or the handler finishes) the decision is made and the rest
// of the body streams through.
type compressWriter struct {
	http.ResponseWriter

	request *http.Request

	status      int
	wroteHeader bool
	decided     bool
	compressing bool
	buffer      bytes.Buffer
	encoder     *gzip.Writer
}

func (w *compressWriter) WriteHeader(status int) {
	// Informational responses (103 Early Hints) are sent immediately and do
	// not end the response.
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.wroteHeader {
		return
	}

	w.status = status
	w.wroteHeader = true
}

func (w *compressWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.decided {
		return w.writeBody(body)
	}

	w.buffer.Write(body)
	if w.buffer.Len() >= minCompressSize {
		if err := w.decide(); err != nil {
			return 0, err
		}
	}

	return len(body), nil
}

// Flush sends whatever is buffered and flushes the encoder, so a handler that
// streams with http.NewResponseController keeps working.
func (w *compressWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if !w.decided {
		_ = w.decide()
	}
	if w.encoder != nil {
		_ = w.encoder.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *compressWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// close finishes the response: it decides for bodies that never reached the
// threshold and writes the gzip trailer when one is open.
func (w *compressWriter) close() {
	if !w.wroteHeader {
		// The handler wrote nothing at all; let net/http produce its own
		// empty 200 response.
		return
	}
	if !w.decided {
		_ = w.decide()
	}
	if w.encoder == nil {
		return
	}

	_ = w.encoder.Close()
	w.encoder.Reset(io.Discard)
	gzipPool.Put(w.encoder)
	w.encoder = nil
}

// decide commits to compressing or not and releases the buffered body. It runs
// at most once, and only before the response header has been sent upstream.
func (w *compressWriter) decide() error {
	if w.decided {
		return nil
	}
	w.decided = true
	w.compressing = w.shouldCompress(w.buffer.Len())

	header := w.ResponseWriter.Header()
	if w.compressing {
		header.Del("Content-Length")
		header.Set("Content-Encoding", "gzip")
	}
	w.ResponseWriter.WriteHeader(w.status)

	body := w.buffer.Bytes()
	w.buffer.Reset()
	if len(body) == 0 || !bodyAllowedForStatus(w.status) {
		return nil
	}
	_, err := w.writeEncoded(body)

	return err
}

// writeBody forwards a body chunk after the decision has been made.
func (w *compressWriter) writeBody(body []byte) (int, error) {
	if !bodyAllowedForStatus(w.status) {
		return len(body), nil
	}

	return w.writeEncoded(body)
}

func (w *compressWriter) writeEncoded(body []byte) (int, error) {
	if !w.compressing {
		return w.ResponseWriter.Write(body)
	}
	if w.encoder == nil {
		w.encoder = gzipPool.Get().(*gzip.Writer)
		w.encoder.Reset(w.ResponseWriter)
	}

	return w.encoder.Write(body)
}

// shouldCompress applies the skip rules to one response.
func (w *compressWriter) shouldCompress(size int) bool {
	if size < minCompressSize {
		return false
	}
	if !bodyAllowedForStatus(w.status) {
		return false
	}

	header := w.ResponseWriter.Header()
	if header.Get("Content-Encoding") != "" {
		return false
	}
	if !acceptsGzip(w.request.Header.Get("Accept-Encoding")) {
		return false
	}

	return compressibleContentType(w.contentType())
}

// contentType returns the handler's Content-Type, sniffing it from the buffer
// when the handler did not set one.
func (w *compressWriter) contentType() string {
	if contentType := w.ResponseWriter.Header().Get("Content-Type"); contentType != "" {
		return contentType
	}

	return http.DetectContentType(w.buffer.Bytes())
}

// bodyAllowedForStatus mirrors net/http's rule for statuses that carry no body.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status < 200:
		return false
	case status == http.StatusNoContent, status == http.StatusNotModified:
		return false
	default:
		return true
	}
}

// acceptsGzip reports whether Accept-Encoding allows gzip. "gzip;q=0" and
// "identity" opt out; "*" accepts it; an absent header means no encoding is
// acceptable.
func acceptsGzip(header string) bool {
	gzipQuality, wildcardQuality := -1.0, -1.0

	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(part, ";")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		switch {
		case strings.EqualFold(name, "gzip"), strings.EqualFold(name, "x-gzip"):
			gzipQuality = quality(params)
		case name == "*":
			wildcardQuality = quality(params)
		}
	}

	if gzipQuality >= 0 {
		return gzipQuality > 0
	}

	return wildcardQuality > 0
}

// quality parses the q-value from one Accept-Encoding parameter list.
func quality(params string) float64 {
	for param := range strings.SplitSeq(params, ";") {
		key, value, ok := strings.Cut(param, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0
		}

		return parsed
	}

	return 1
}

// compressibleContentType reports whether the media type benefits from gzip.
// Images (except SVG), fonts, audio, video, archives and event streams are
// already compressed, binary, or streamed incrementally.
func compressibleContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		mediaType, _, _ = strings.Cut(value, ";")
		mediaType = strings.TrimSpace(mediaType)
	}
	mediaType = strings.ToLower(mediaType)

	if mediaType == "image/svg+xml" {
		return true
	}
	if mediaType == "text/event-stream" {
		return false
	}

	for _, prefix := range nonCompressibleTypes {
		if strings.HasPrefix(mediaType, prefix) {
			return false
		}
	}

	return true
}

// nonCompressibleTypes are the media type prefixes that pass through untouched.
var nonCompressibleTypes = []string{
	"image/",
	"font/",
	"audio/",
	"video/",
	"application/zip",
	"application/gzip",
	"application/x-gzip",
	"application/x-brotli",
	"application/zstd",
	"application/pdf",
	"application/wasm",
	"application/octet-stream",
}

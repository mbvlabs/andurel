package anduin

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/pkg/telemetry"
)

// Recover converts a panic in a downstream handler into a 500 error so the
// router's ErrorHandler can render it and the process keeps serving.
func Recover() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.Error("anduin: recovered panic",
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					err = &HTTPError{
						Status:  http.StatusInternalServerError,
						Message: "internal server error",
					}
				}
			}()
			return next(ctx)
		}
	}
}

// Logger logs one line per completed request. Prefixes listed in skipPrefixes
// are not logged (assets and API traffic, for example).
func Logger(log *slog.Logger, skipPrefixes ...string) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			request := Request(ctx)
			if request == nil || pathHasPrefix(request.URL.Path, skipPrefixes) {
				return next(ctx)
			}

			recorder := &statusWriter{ResponseWriter: ResponseWriter(ctx)}
			ctx = ReplaceResponseWriter(ctx, recorder)

			start := time.Now()
			err := next(ctx)
			duration := time.Since(start)

			if log != nil {
				status := recorder.status
				if status == 0 {
					status = StatusCode(err)
					if err == nil {
						status = http.StatusOK
					}
				}
				log.InfoContext(ctx, "HTTP request completed",
					"method", request.Method,
					"path", request.URL.Path,
					"status", status,
					"duration", duration.String(),
				)
			}
			return err
		}
	}
}

// Telemetry attaches the telemetry pipeline to the request context and records
// the matched route on the active span and metric labels.
func Telemetry(tel *telemetry.Telemetry) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			if tel != nil {
				ctx = tel.Context(ctx)
			}
			err := next(ctx)
			if pattern := RoutePattern(ctx); pattern != "" {
				telemetry.SetHTTPRoute(ctx, RoutePath(pattern))
			}
			return err
		}
	}
}

// RoutePath strips the method prefix from an http.ServeMux pattern, yielding
// the request path shape (for example "GET /users/{id}" -> "/users/{id}").
func RoutePath(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if method, rest, ok := strings.Cut(pattern, " "); ok && isHTTPMethod(method) {
		return rest
	}
	return pattern
}

func isHTTPMethod(value string) bool {
	switch value {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions,
		http.MethodTrace:
		return true
	default:
		return false
	}
}

func pathHasPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
		if prefix == "" {
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// statusWriter records the status code and byte count of a response so logging
// middleware can report them after the handler returns.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(body)
	w.bytes += written
	return written, err
}

func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

var (
	_ http.Flusher                              = (*statusWriter)(nil)
	_ interface{ Unwrap() http.ResponseWriter } = (*statusWriter)(nil)
)

package anduin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) recordsFor(message string) []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var matches []slog.Record
	for _, record := range h.records {
		if record.Message == message {
			matches = append(matches, record)
		}
	}
	return matches
}

func captureDefaultLogger(t *testing.T) *captureHandler {
	t.Helper()
	capture := &captureHandler{}
	previous := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return capture
}

func attribute(record slog.Record, key string) string {
	var value string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			value = attr.Value.String()
			return false
		}
		return true
	})
	return value
}

func assertRecordOmits(t *testing.T, record slog.Record, secret string) {
	t.Helper()
	if strings.Contains(record.Message, secret) {
		t.Fatalf("log message leaks %q: %q", secret, record.Message)
	}
	record.Attrs(func(attr slog.Attr) bool {
		if strings.Contains(attr.Value.String(), secret) {
			t.Fatalf("log attribute %q leaks %q: %q", attr.Key, secret, attr.Value.String())
		}
		return true
	})
}

func TestLoggerLogsMatchedRouteInsteadOfRawPath(t *testing.T) {
	capture := &captureHandler{}
	router := NewRouter()
	router.Use(Logger(slog.New(capture)))
	router.Get("/password/:token/edit", func(context.Context) error { return nil })

	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/password/secret-token-value/edit")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer response.Body.Close()

	records := capture.recordsFor("HTTP request completed")
	if len(records) != 1 {
		t.Fatalf("request records = %d, want 1", len(records))
	}
	if path := attribute(records[0], "path"); path != "/password/{token}/edit" {
		t.Fatalf("logged path = %q, want the route template", path)
	}
	assertRecordOmits(t, records[0], "secret-token-value")
}

func TestDefaultErrorHandlerLogsMatchedRouteInsteadOfRawPath(t *testing.T) {
	capture := captureDefaultLogger(t)
	router := NewRouter()
	router.Get("/password/:token/edit", func(context.Context) error { return ErrNotFound })

	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/password/secret-token-value/edit")
	if err != nil {
		t.Fatalf("matched GET: %v", err)
	}
	response.Body.Close()
	response, err = http.Get(server.URL + "/missing/secret-token-value")
	if err != nil {
		t.Fatalf("unmatched GET: %v", err)
	}
	response.Body.Close()

	records := capture.recordsFor("anduin: handler error")
	if len(records) != 2 {
		t.Fatalf("handler error records = %d, want 2", len(records))
	}
	if path := attribute(records[0], "path"); path != "/password/{token}/edit" {
		t.Fatalf("matched request path = %q, want the route template", path)
	}
	if path := attribute(records[1], "path"); path != "" {
		t.Fatalf("unmatched request path = %q, want the empty route", path)
	}
	for _, record := range records {
		assertRecordOmits(t, record, "secret-token-value")
	}
}

package inertia

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
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

func routeRequest(target, pattern string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Pattern = pattern
	return request
}

func TestProtocolPageLogsRouteInsteadOfSecretPath(t *testing.T) {
	capture := captureDefaultLogger(t)
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)), WithProtocolDebug(true))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := routeRequest("/password/secret-token-value/edit", "GET /password/{token}/edit")
	if err := renderer.Page(testContext(request, httptest.NewRecorder()), "Auth/ResetPassword", Props{}).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	records := capture.recordsFor("inertia protocol page")
	if len(records) != 1 {
		t.Fatalf("page records = %d, want 1", len(records))
	}
	if url := attribute(records[0], "url"); url != "/password/{token}/edit" {
		t.Fatalf("logged url = %q, want the route template", url)
	}
	assertRecordOmits(t, records[0], "secret-token-value")
}

func TestSSRFallbackLogsRouteInsteadOfSecretPath(t *testing.T) {
	capture := captureDefaultLogger(t)
	var captured RootData
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(&fakeSSRRenderer{err: errors.New("SSR unavailable")}),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := routeRequest("/password/secret-token-value/edit", "GET /password/{token}/edit")
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Auth/ResetPassword", nil).SSR().Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	records := capture.recordsFor("inertia SSR fallback")
	if len(records) != 1 {
		t.Fatalf("fallback records = %d, want 1", len(records))
	}
	if url := attribute(records[0], "url"); url != "/password/{token}/edit" {
		t.Fatalf("logged url = %q, want the route template", url)
	}
	assertRecordOmits(t, records[0], "secret-token-value")
}

func TestRescuedDeferredPropLogsRouteInsteadOfSecretPath(t *testing.T) {
	capture := captureDefaultLogger(t)
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := routeRequest("/password/secret-token-value/edit", "GET /password/{token}/edit")
	request.Header.Set(HeaderInertia, "true")
	request.Header.Set(HeaderPartialComponent, "Auth/ResetPassword")
	request.Header.Set(HeaderPartialData, "preview")
	props := Props{
		"preview": Deferred(
			func(context.Context) (any, error) { return nil, errors.New("preview unavailable") },
			Rescue(),
		),
	}
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Auth/ResetPassword", props).Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	records := capture.recordsFor("inertia rescued deferred prop")
	if len(records) != 1 {
		t.Fatalf("rescued records = %d, want 1", len(records))
	}
	if url := attribute(records[0], "url"); url != "/password/{token}/edit" {
		t.Fatalf("logged url = %q, want the route template", url)
	}
	assertRecordOmits(t, records[0], "secret-token-value")
}

func TestProtocolRequestLogsRouteInsteadOfSecretPath(t *testing.T) {
	capture := captureDefaultLogger(t)
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)), WithProtocolDebug(true))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := routeRequest("/password/secret-token-value/edit", "")
	handler := renderer.Middleware()(func(ctx context.Context) error {
		anduin.Request(ctx).Pattern = "GET /password/{token}/edit"
		return nil
	})
	recorder := httptest.NewRecorder()
	if err := handler(testContext(request, recorder)); err != nil {
		t.Fatalf("middleware: %v", err)
	}

	records := capture.recordsFor("inertia protocol request")
	if len(records) != 1 {
		t.Fatalf("protocol records = %d, want 1", len(records))
	}
	if url := attribute(records[0], "url"); url != "/password/{token}/edit" {
		t.Fatalf("logged url = %q, want the route template", url)
	}
	assertRecordOmits(t, records[0], "secret-token-value")
}

func TestProtocolRequestLogsNoPathWhenUnmatched(t *testing.T) {
	capture := captureDefaultLogger(t)
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)), WithProtocolDebug(true))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := routeRequest("/missing/secret-token-value", "")
	handler := renderer.Middleware()(func(context.Context) error { return nil })
	recorder := httptest.NewRecorder()
	if err := handler(testContext(request, recorder)); err != nil {
		t.Fatalf("middleware: %v", err)
	}

	records := capture.recordsFor("inertia protocol request")
	if len(records) != 1 {
		t.Fatalf("protocol records = %d, want 1", len(records))
	}
	if url := attribute(records[0], "url"); url != "" {
		t.Fatalf("logged url = %q, want the empty route", url)
	}
	assertRecordOmits(t, records[0], "secret-token-value")
}

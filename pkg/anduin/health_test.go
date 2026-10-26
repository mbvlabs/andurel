package anduin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testHealth struct {
	err error
}

func (h testHealth) Health(context.Context) error {
	return h.err
}

func TestHealthAnswersJSONWhenHealthy(t *testing.T) {
	router := NewRouter()
	router.Get("/health", Health(testHealth{}))

	recorder := httptest.NewRecorder()
	router.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Body.String(); got != `"app is healthy and running"` {
		t.Fatalf("body = %q", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestHealthAnswers503WhenCheckerFails(t *testing.T) {
	router := NewRouter()
	router.Get("/health", Health(testHealth{err: errors.New("database down")}))

	recorder := httptest.NewRecorder()
	router.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if got := recorder.Body.String(); got != "Service Unavailable\n" {
		t.Fatalf("body = %q", got)
	}
}

package anduin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestJSONErrorHandlerReceivesMethodNotAllowed covers the API contract behind
// the method-not-allowed manual action: a custom ErrorHandler that writes JSON
// still sees anduin.ErrMethodNotAllowed, can classify it as 405 through
// StatusCode, and inherits the Allow header http.ServeMux set.
func TestJSONErrorHandlerReceivesMethodNotAllowed(t *testing.T) {
	t.Parallel()

	var received error
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		received = err
		return JSON(ctx, StatusCode(err), map[string]string{"error": "method not allowed"})
	}))
	rt.Get("/api/documentations/v1", func(ctx context.Context) error {
		return NoContent(ctx, http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/documentations/v1", nil))

	if !errors.Is(received, ErrMethodNotAllowed) {
		t.Fatalf("ErrorHandler err = %v, want ErrMethodNotAllowed", received)
	}
	if got := StatusCode(received); got != http.StatusMethodNotAllowed {
		t.Fatalf("StatusCode(err) = %d, want 405", got)
	}
	if got, want := recorder.Code, http.StatusMethodNotAllowed; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := recorder.Header().Get("Allow"), "GET, HEAD"; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
	if got, want := recorder.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode JSON body: %v\n%s", err, recorder.Body.String())
	}
	if got, want := body["error"], "method not allowed"; got != want {
		t.Fatalf("body error = %q, want %q", got, want)
	}
}

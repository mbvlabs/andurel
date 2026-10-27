package anduin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlobWritesBodyWithExplicitContentType(t *testing.T) {
	body := []byte("%PDF-1.7\n")

	recorder := httptest.NewRecorder()
	ctx := ReplaceResponseWriter(context.Background(), recorder)

	if err := Blob(ctx, http.StatusAccepted, "application/pdf", body); err != nil {
		t.Fatalf("Blob: %v", err)
	}

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/pdf")
	}
	if got := recorder.Body.String(); got != string(body) {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestBlobWithoutResponseWriterReturns500(t *testing.T) {
	err := Blob(context.Background(), http.StatusOK, "application/pdf", []byte("body"))
	if err == nil {
		t.Fatal("Blob error = nil, want a missing-writer error")
	}
	if got := StatusCode(err); got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", got)
	}
}

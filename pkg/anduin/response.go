package anduin

import (
	"context"
	"encoding/json"
	"net/http"
)

// SetHeader sets a response header.
func SetHeader(ctx context.Context, key, value string) {
	if writer := ResponseWriter(ctx); writer != nil {
		writer.Header().Set(key, value)
	}
}

// AddHeader appends a response header value.
func AddHeader(ctx context.Context, key, value string) {
	if writer := ResponseWriter(ctx); writer != nil {
		writer.Header().Add(key, value)
	}
}

// JSON writes v as a JSON response body.
func JSON(ctx context.Context, status int, v any) error {
	blob, err := json.Marshal(v)
	if err != nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: encode JSON", Err: err}
	}
	return JSONBlob(ctx, status, blob)
}

// JSONBlob writes an already-encoded JSON body.
func JSONBlob(ctx context.Context, status int, blob []byte) error {
	return writeBody(ctx, status, "application/json; charset=utf-8", blob)
}

// Blob writes body with an explicit content type, for responses that are
// neither JSON, text, nor HTML (PDFs, images, CSV exports).
func Blob(ctx context.Context, status int, contentType string, body []byte) error {
	return writeBody(ctx, status, contentType, body)
}

// Text writes a plain-text response body.
func Text(ctx context.Context, status int, value string) error {
	return writeBody(ctx, status, "text/plain; charset=utf-8", []byte(value))
}

// HTML writes an HTML response body.
func HTML(ctx context.Context, status int, value string) error {
	return writeBody(ctx, status, "text/html; charset=utf-8", []byte(value))
}

// HTMLBlob writes an already-rendered HTML body.
func HTMLBlob(ctx context.Context, status int, blob []byte) error {
	return writeBody(ctx, status, "text/html; charset=utf-8", blob)
}

// NoContent writes a response with the given status and no body.
func NoContent(ctx context.Context, status int) error {
	writer := ResponseWriter(ctx)
	if writer == nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: no response writer on context"}
	}
	writer.WriteHeader(status)
	return nil
}

// Redirect writes a redirect response.
func Redirect(ctx context.Context, location string, status int) error {
	if status < 300 || status > 399 {
		return &HTTPError{
			Status:  http.StatusInternalServerError,
			Message: "anduin: redirect status must be 3xx",
		}
	}
	writer := ResponseWriter(ctx)
	if writer == nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: no response writer on context"}
	}
	writer.Header().Set("Location", location)
	writer.WriteHeader(status)
	return nil
}

func writeBody(ctx context.Context, status int, contentType string, body []byte) error {
	writer := ResponseWriter(ctx)
	if writer == nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: no response writer on context"}
	}
	if contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	writer.WriteHeader(status)
	if len(body) == 0 {
		return nil
	}
	if _, err := writer.Write(body); err != nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: write response", Err: err}
	}
	return nil
}

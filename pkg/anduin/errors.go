package anduin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

// ErrNotFound is the sentinel the router passes to the ErrorHandler when no
// route matches a request. Applications map it to their themed 404 page.
var ErrNotFound = errors.New("anduin: not found")

// ErrMethodNotAllowed is the sentinel the router passes to the ErrorHandler
// when a route matches the path but not the request method. The Allow header
// set by http.ServeMux is preserved on the response. Applications map it to
// their themed page.
var ErrMethodNotAllowed = errors.New("anduin: method not allowed")

// HTTPError carries an HTTP status alongside an optional cause. Handlers return
// it (directly or through Error) and the router's ErrorHandler maps it.
type HTTPError struct {
	Status  int
	Message string
	Err     error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.Status)
}

// Unwrap returns the underlying cause, if any.
func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Error annotates err with an HTTP status.
func Error(_ context.Context, status int, err error) error {
	return &HTTPError{Status: status, Err: err}
}

// Errorf builds an *HTTPError with a formatted message.
func Errorf(status int, format string, args ...any) error {
	return &HTTPError{Status: status, Message: fmt.Sprintf(format, args...)}
}

// NotFound annotates err as an HTTP 404.
func NotFound(err error) error {
	return &HTTPError{Status: http.StatusNotFound, Err: err}
}

// BadRequest annotates err as an HTTP 400.
func BadRequest(err error) error {
	return &HTTPError{Status: http.StatusBadRequest, Err: err}
}

// Unauthorized annotates err as an HTTP 401.
func Unauthorized(err error) error {
	return &HTTPError{Status: http.StatusUnauthorized, Err: err}
}

// Forbidden annotates err as an HTTP 403.
func Forbidden(err error) error {
	return &HTTPError{Status: http.StatusForbidden, Err: err}
}

// Conflict annotates err as an HTTP 409.
func Conflict(err error) error {
	return &HTTPError{Status: http.StatusConflict, Err: err}
}

// StatusCode reports the HTTP status associated with err. An unclassified error
// is a 500.
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, ErrMethodNotAllowed) {
		return http.StatusMethodNotAllowed
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Status != 0 {
		return httpErr.Status
	}

	return http.StatusInternalServerError
}

// Message returns the client-facing message for err, falling back to the
// standard status text for its code.
func Message(err error) string {
	if err == nil {
		return ""
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Message != "" {
		return httpErr.Message
	}
	return http.StatusText(StatusCode(err))
}

// defaultErrorHandler is the ErrorHandler used when the router is not given one.
// It writes the status and message as plain text.
func defaultErrorHandler(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	status := StatusCode(err)
	if status == http.StatusInternalServerError {
		slog.Error("anduin: unhandled handler error", "error", err)
	}
	if request := Request(ctx); request != nil {
		slog.Debug("anduin: handler error",
			"method", request.Method,
			"path", request.URL.Path,
			"status", strconv.Itoa(status),
			"error", err,
		)
	}
	writer := ResponseWriter(ctx)
	if writer == nil {
		return nil
	}
	http.Error(writer, http.StatusText(status), status)
	return nil
}

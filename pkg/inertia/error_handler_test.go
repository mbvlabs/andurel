package inertia

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

type testFieldError struct {
	fields map[string]string
}

func (e testFieldError) Error() string {
	return "validation failed"
}

func (e testFieldError) ToMap() map[string]string {
	return e.fields
}

func errorHandlerRequest(method, target string, headers map[string]string) (*http.Request, *httptest.ResponseRecorder, context.Context) {
	request := httptest.NewRequest(method, target, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()

	return request, recorder, testContext(request, recorder)
}

func TestErrorHandlerRendersFieldErrorsAsValidationPage(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	_, recorder, ctx := errorHandlerRequest(
		http.MethodPost,
		"/login",
		map[string]string{HeaderInertia: "true"},
	)
	handler := ErrorHandler(renderer)
	fieldErr := testFieldError{fields: map[string]string{"email": "is required"}}

	if err := handler(ctx, fieldErr); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	body := recorder.Body.String()
	for _, want := range []string{`"component":"Errors/Validation"`, `"email":"is required"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %s: %s", want, body)
		}
	}
}

func TestErrorHandlerWritesJSONFieldErrors(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	_, recorder, ctx := errorHandlerRequest(
		http.MethodPost,
		"/login",
		map[string]string{"Accept": "application/json"},
	)
	handler := ErrorHandler(renderer)
	fieldErr := testFieldError{fields: map[string]string{"email": "is required"}}

	if err := handler(ctx, fmt.Errorf("wrapped: %w", fieldErr)); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	want := `{"error":"Unprocessable Entity","fields":{"email":"is required"}}`
	if got := strings.TrimSpace(recorder.Body.String()); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestErrorHandlerMapsSentinelsToPagesAndJSON(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	sentinel := errors.New("not found")
	handler := ErrorHandler(renderer, MapError(sentinel, http.StatusNotFound, "Errors/NotFound"))

	_, recorder, ctx := errorHandlerRequest(
		http.MethodGet,
		"/users/1",
		map[string]string{HeaderInertia: "true"},
	)
	if err := handler(ctx, fmt.Errorf("lookup: %w", sentinel)); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"component":"Errors/NotFound"`) {
		t.Fatalf("body = %s", body)
	}

	_, recorder, ctx = errorHandlerRequest(
		http.MethodGet,
		"/users/1",
		map[string]string{"Accept": "application/json"},
	)
	if err := handler(ctx, sentinel); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"error":"Not Found"}` {
		t.Fatalf("body = %q", got)
	}
}

func TestErrorHandlerAcceptsMappingSlice(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	sentinel := errors.New("not found")
	mappings := []ErrorMapping{
		MapError(sentinel, http.StatusNotFound, "Errors/NotFound"),
	}
	handler := ErrorHandler(renderer, mappings...)

	_, recorder, ctx := errorHandlerRequest(http.MethodGet, "/users/1", nil)
	if err := handler(ctx, sentinel); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestErrorHandlerPassesUnmappedErrorsThrough(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	handler := ErrorHandler(renderer)

	unmapped := errors.New("unmapped")

	_, recorder, ctx := errorHandlerRequest(
		http.MethodGet,
		"/users/1",
		map[string]string{HeaderInertia: "true"},
	)
	if got := handler(ctx, unmapped); got != unmapped {
		t.Fatalf("ErrorHandler returned %v, want the original error", got)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}

	_, recorder, ctx = errorHandlerRequest(
		http.MethodGet,
		"/users/1",
		map[string]string{"Accept": "application/json"},
	)
	if got := handler(ctx, unmapped); got != unmapped {
		t.Fatalf("ErrorHandler returned %v, want the original error", got)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("5xx body = %q, want empty", recorder.Body.String())
	}
}

func TestErrorHandlerWritesUnmappedClientErrorAsJSON(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	_, recorder, ctx := errorHandlerRequest(
		http.MethodPost,
		"/login",
		map[string]string{"Accept": "application/json"},
	)
	if err := ErrorHandler(renderer)(ctx, anduin.BadRequest(errors.New("bad body"))); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"error":"Bad Request"}` {
		t.Fatalf("body = %q", got)
	}
}

func TestErrorHandlerSSRValidationPage(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">validation</div>`,
	}}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	_, recorder, ctx := errorHandlerRequest(http.MethodPost, "/login", nil)
	handler := NewErrorHandler(
		renderer,
		ErrorHandlerOptions{ValidationSSR: true},
	)
	fieldErr := testFieldError{fields: map[string]string{"email": "is required"}}

	if err := handler(ctx, fieldErr); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if ssr.calls != 1 || captured.SSR == nil {
		t.Fatalf("SSR calls = %d, captured = %#v", ssr.calls, captured.SSR)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "validation") {
		t.Fatalf("body = %s", body)
	}
}

func TestErrorHandlerSSRMappedPage(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Body: `<div data-server-rendered="true" data-page="app">not found</div>`,
	}}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	sentinel := errors.New("not found")
	mapping := MapError(sentinel, http.StatusNotFound, "Errors/NotFound")
	mapping.SSR = true
	handler := ErrorHandler(renderer, mapping)

	_, recorder, ctx := errorHandlerRequest(http.MethodGet, "/users/1", nil)
	if err := handler(ctx, sentinel); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if ssr.calls != 1 || captured.SSR == nil {
		t.Fatalf("SSR calls = %d, captured = %#v", ssr.calls, captured.SSR)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "not found") {
		t.Fatalf("body = %s", body)
	}
}

func TestErrorHandlerRendersFallbackPageForUnmappedServerError(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	handler := NewErrorHandler(
		renderer,
		ErrorHandlerOptions{FallbackComponent: "Errors/InternalError"},
	)

	_, recorder, ctx := errorHandlerRequest(http.MethodGet, "/dashboard", nil)
	if err := handler(ctx, errors.New("boom")); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if captured.Page.Component != "Errors/InternalError" {
		t.Fatalf("component = %q, want Errors/InternalError", captured.Page.Component)
	}
}

func TestErrorHandlerFallbackPageLeavesClientErrorsToTheRouter(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	handler := NewErrorHandler(
		renderer,
		ErrorHandlerOptions{FallbackComponent: "Errors/InternalError"},
	)
	badRequest := anduin.BadRequest(errors.New("bad body"))

	_, recorder, ctx := errorHandlerRequest(http.MethodPost, "/login", nil)
	if got := handler(ctx, badRequest); got != badRequest {
		t.Fatalf("ErrorHandler returned %v, want the original error", got)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestErrorHandlerFallbackPageDoesNotSSRAnSSRFailure(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{err: errors.New("sidecar down")}
	renderer, err := newTestRenderer(
		WithRoot(testRoot(&captured)),
		WithSSRRenderer(ssr),
		WithSSRFailFast(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	ctx := testContext(request, recorder)
	if err := renderer.Page(ctx, "Dashboard", Props{}).SSR().Render(); err == nil {
		t.Fatal("expected SSR failure")
	}

	handler := NewErrorHandler(
		renderer,
		ErrorHandlerOptions{
			FallbackComponent: "Errors/InternalError",
			FallbackSSR:       true,
		},
	)
	ssr.calls = 0
	_, recorder, ctx = errorHandlerRequest(http.MethodGet, "/dashboard", nil)
	if err := handler(ctx, &Error{Kind: ErrorSSR, Err: errors.New("sidecar down")}); err != nil {
		t.Fatalf("ErrorHandler: %v", err)
	}
	if ssr.calls != 0 {
		t.Fatalf("fallback page rendered through SSR %d times", ssr.calls)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if captured.Page.Component != "Errors/InternalError" {
		t.Fatalf("component = %q, want Errors/InternalError", captured.Page.Component)
	}
}

func TestPageErrorsSetsFieldMap(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	_, recorder, ctx := errorHandlerRequest(
		http.MethodPost,
		"/login",
		map[string]string{HeaderInertia: "true"},
	)
	page, ok := renderer.Page(ctx, "Auth/Login", Props{}).Errors(
		testFieldError{fields: map[string]string{"password": "is required"}},
	)
	if !ok {
		t.Fatal("Errors reported no field errors")
	}
	if err := page.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"password":"is required"`) {
		t.Fatalf("body = %s", body)
	}

	page, ok = renderer.Page(ctx, "Auth/Login", Props{}).Errors(errors.New("plain"))
	if ok {
		t.Fatal("Errors reported field errors for a plain error")
	}
	if page == nil {
		t.Fatal("Errors returned a nil page")
	}
}

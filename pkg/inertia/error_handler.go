package inertia

import (
	"context"
	"errors"
	"html"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

const validationErrorComponent = "Errors/Validation"

// ErrorMapping maps a sentinel error to an Inertia error page.
type ErrorMapping struct {
	Target    error
	Status    int
	Component string
	// SSR renders this error page server-side. Every error page is
	// client-rendered unless it opts in here (or through ValidationSSR and
	// FallbackSSR), even when the renderer defaults pages to SSR.
	SSR bool
}

// MapError builds an ErrorMapping for an ErrorHandler.
func MapError(target error, status int, component string) ErrorMapping {
	return ErrorMapping{
		Target:    target,
		Status:    status,
		Component: component,
	}
}

// ErrorHandlerOptions configures the pages ErrorHandler builds itself. Zero
// values keep the defaults: client-rendered Errors/Validation and no fallback
// page.
type ErrorHandlerOptions struct {
	// ValidationComponent overrides the component used for field errors.
	ValidationComponent string
	// ValidationSSR renders validation responses server-side.
	ValidationSSR bool
	// FallbackComponent, when set, is the themed page for unmapped 5xx errors,
	// including a failed SSR render with fail-fast enabled.
	FallbackComponent string
	// FallbackSSR renders the fallback page server-side. It stays
	// client-rendered when the error itself came from SSR so the error page
	// cannot fail the same way.
	FallbackSSR bool
}

// ErrorHandler translates handler errors into a JSON or Inertia response.
// Field errors become a 422 page; mapped sentinels become their configured
// page; unmapped errors keep their own status. An unmapped 5xx document
// request without a configured fallback component is answered by a built-in
// escaped HTML page, so handler details are never leaked. Use NewErrorHandler
// to configure the validation and fallback pages.
//
// Error pages are client-rendered unless the mapping or options opt them into
// SSR, even when the renderer defaults every other page to SSR. That keeps a
// failed SSR render from being retried for the error page it caused.
func ErrorHandler(renderer *Renderer, mappings ...ErrorMapping) anduin.ErrorHandler {
	return NewErrorHandler(renderer, ErrorHandlerOptions{}, mappings...)
}

// NewErrorHandler is ErrorHandler with options for the pages it builds itself.
// Configure FallbackComponent to render unmapped 5xx errors as a themed page
// instead of the built-in safe HTML page.
func NewErrorHandler(
	renderer *Renderer,
	options ErrorHandlerOptions,
	mappings ...ErrorMapping,
) anduin.ErrorHandler {
	config := errorHandlerConfig{
		validationComponent: validationErrorComponent,
		validationSSR:       options.ValidationSSR,
		fallbackComponent:   options.FallbackComponent,
		fallbackSSR:         options.FallbackSSR,
		mappings:            mappings,
	}
	if options.ValidationComponent != "" {
		config.validationComponent = options.ValidationComponent
	}

	return func(ctx context.Context, err error) error {
		if wantsJSON(ctx) {
			return writeJSONError(ctx, err, config.mappings)
		}

		if fields, ok := fieldErrors(err); ok {
			return config.validationPage(renderer, ctx, fields)
		}

		for _, mapping := range config.mappings {
			if mapping.Target != nil && errors.Is(err, mapping.Target) {
				return config.mappedPage(renderer, ctx, mapping)
			}
		}

		if config.fallbackComponent != "" &&
			anduin.StatusCode(err) >= http.StatusInternalServerError {
			return config.fallbackPage(renderer, ctx, err)
		}

		if anduin.StatusCode(err) >= http.StatusInternalServerError {
			return safeErrorPage(ctx, err)
		}

		return err
	}
}

type errorHandlerConfig struct {
	validationComponent string
	validationSSR       bool
	fallbackComponent   string
	fallbackSSR         bool
	mappings            []ErrorMapping
}

func (config errorHandlerConfig) validationPage(
	renderer *Renderer,
	ctx context.Context,
	fields map[string]string,
) error {
	page := renderer.Page(ctx, config.validationComponent, Props{}).
		ValidationErrors(fields).
		Status(http.StatusUnprocessableEntity)
	page = applyErrorSSRPolicy(page, config.validationSSR)

	return page.Render()
}

func (config errorHandlerConfig) mappedPage(
	renderer *Renderer,
	ctx context.Context,
	mapping ErrorMapping,
) error {
	page := renderer.Page(ctx, mapping.Component, Props{}).Status(mapping.Status)
	page = applyErrorSSRPolicy(page, mapping.SSR)

	return page.Render()
}

func (config errorHandlerConfig) fallbackPage(
	renderer *Renderer,
	ctx context.Context,
	err error,
) error {
	// The router only logs errors it renders itself; keep 5xx visible.
	slog.ErrorContext(ctx, "inertia: unhandled handler error", "error", err)

	page := renderer.Page(ctx, config.fallbackComponent, Props{}).
		Status(http.StatusInternalServerError)
	page = applyErrorSSRPolicy(page, config.fallbackSSR && !isSSRError(err))

	return page.Render()
}

// applyErrorSSRPolicy makes the error page's SSR decision explicit. Without
// the explicit CSR(), a renderer-wide SSR default would server-render error
// pages too, including the page that answers a failed SSR render.
func applyErrorSSRPolicy(page *PageBuilder, ssr bool) *PageBuilder {
	if ssr {
		return page.SSR()
	}

	return page.CSR()
}

// safeErrorPage answers an unmapped 5xx document request when the application
// configured no fallback component. It never builds an Inertia page: it cannot
// refer to a component that may not exist and cannot recurse into SSR. The body
// is the escaped status text, so handler details are never leaked.
func safeErrorPage(ctx context.Context, err error) error {
	slog.ErrorContext(ctx, "inertia: unhandled handler error", "error", err)

	status := max(anduin.StatusCode(err), http.StatusInternalServerError)
	message := html.EscapeString(http.StatusText(status))
	document := "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">" +
		"<title>" + message + "</title></head><body><h1>" + message + "</h1></body></html>"

	return anduin.HTML(ctx, status, document)
}

func isSSRError(err error) bool {
	var inertiaErr *Error
	return errors.As(err, &inertiaErr) && inertiaErr.Kind == ErrorSSR
}

// wantsJSON reports whether the request is an API call rather than an Inertia
// visit; Inertia visits send X-Inertia and accept HTML.
func wantsJSON(ctx context.Context) bool {
	return anduin.Header(ctx, HeaderInertia) == "" &&
		strings.Contains(anduin.Header(ctx, "Accept"), "application/json")
}

func writeJSONError(ctx context.Context, err error, mappings []ErrorMapping) error {
	if fields, ok := fieldErrors(err); ok {
		return anduin.JSON(ctx, http.StatusUnprocessableEntity, map[string]any{
			"error":  http.StatusText(http.StatusUnprocessableEntity),
			"fields": fields,
		})
	}

	status := anduin.StatusCode(err)
	for _, mapping := range mappings {
		if mapping.Target != nil && errors.Is(err, mapping.Target) {
			status = mapping.Status
			break
		}
	}
	if status >= http.StatusInternalServerError {
		// The router logs and answers unexpected failures without leaking details.
		return err
	}

	return anduin.JSON(ctx, status, map[string]string{"error": http.StatusText(status)})
}

// fieldErrors reports whether err carries a field map. The interface keeps this
// package independent of the validation implementation.
func fieldErrors(err error) (map[string]string, bool) {
	var fieldErr interface{ ToMap() map[string]string }
	if errors.As(err, &fieldErr) {
		return fieldErr.ToMap(), true
	}

	return nil, false
}

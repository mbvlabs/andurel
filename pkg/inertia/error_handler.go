package inertia

import (
	"context"
	"errors"
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
}

// MapError builds an ErrorMapping for an ErrorHandler.
func MapError(target error, status int, component string) ErrorMapping {
	return ErrorMapping{
		Target:    target,
		Status:    status,
		Component: component,
	}
}

// ErrorHandler translates handler errors into a JSON or Inertia response.
// Field errors become a 422 page; mapped sentinels become their configured
// page; unmapped errors keep their own status, and 5xx errors are passed back
// to the router so their details are never leaked.
func ErrorHandler(renderer *Renderer, mappings ...ErrorMapping) anduin.ErrorHandler {
	return func(ctx context.Context, err error) error {
		if wantsJSON(ctx) {
			return writeJSONError(ctx, err, mappings)
		}

		if fields, ok := fieldErrors(err); ok {
			return renderer.Page(ctx, validationErrorComponent, Props{}).
				ValidationErrors(fields).
				Status(http.StatusUnprocessableEntity).
				Render()
		}

		for _, mapping := range mappings {
			if mapping.Target != nil && errors.Is(err, mapping.Target) {
				return renderer.Page(ctx, mapping.Component, Props{}).
					Status(mapping.Status).
					Render()
			}
		}

		return err
	}
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

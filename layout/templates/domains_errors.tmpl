// Package domains holds the cross-domain vocabulary shared by every domain
// package: the sentinel errors callers match with errors.Is and the
// field-level ValidationError that handlers map to 422 responses and Inertia
// props.errors. It depends on nothing inside the application.
package domains

import "errors"

var (
	ErrNotFound  = errors.New("domain: not found")
	ErrConflict  = errors.New("domain: conflict")
	ErrViolation = errors.New("domain: violation")
)

// ValidationError carries field-level messages that handlers map to 422 /
// Inertia props.errors. It reports true for errors.Is(err, ErrViolation).
type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return "domain: validation failed"
}

func (e ValidationError) Is(target error) bool {
	return target == ErrViolation
}

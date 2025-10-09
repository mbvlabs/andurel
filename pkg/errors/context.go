package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorContext provides context for error wrapping
type ErrorContext struct {
	Operation string                 `json:"operation"`
	Resource  string                 `json:"resource"`
	File      string                 `json:"file"`
	Details   map[string]interface{} `json:"details"`
}

// NewErrorContext creates a new error context
func NewErrorContext(operation, resource, file string) *ErrorContext {
	return &ErrorContext{
		Operation: operation,
		Resource:  resource,
		File:      file,
		Details:   make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the error context
func (ec *ErrorContext) WithDetail(key string, value interface{}) *ErrorContext {
	ec.Details[key] = value
	return ec
}

// WithCaller adds caller information to the error context
func (ec *ErrorContext) WithCaller(skip int) *ErrorContext {
	_, file, line, ok := runtime.Caller(skip + 1)
	if ok {
		ec.Details["caller"] = fmt.Sprintf("%s:%d", file, line)
	}
	return ec
}

// ContextualError provides enhanced error information with context
type ContextualError struct {
	Context *ErrorContext `json:"context"`
	Cause   error         `json:"cause"`
}

// Error implements the error interface
func (e *ContextualError) Error() string {
	var parts []string

	if e.Context.Operation != "" {
		parts = append(parts, fmt.Sprintf("operation: %s", e.Context.Operation))
	}

	if e.Context.Resource != "" {
		parts = append(parts, fmt.Sprintf("resource: %s", e.Context.Resource))
	}

	if e.Context.File != "" {
		parts = append(parts, fmt.Sprintf("file: %s", e.Context.File))
	}

	if len(e.Context.Details) > 0 {
		var details []string
		for k, v := range e.Context.Details {
			details = append(details, fmt.Sprintf("%s: %v", k, v))
		}
		parts = append(parts, fmt.Sprintf("details: [%s]", strings.Join(details, ", ")))
	}

	contextStr := strings.Join(parts, ", ")

	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", contextStr, e.Cause)
	}
	return contextStr
}

// Unwrap returns the underlying cause
func (e *ContextualError) Unwrap() error {
	return e.Cause
}

// WrapError wraps an error with context
func WrapError(err error, ctx ErrorContext) error {
	if err == nil {
		return nil
	}
	return &ContextualError{
		Context: &ctx,
		Cause:   err,
	}
}

// WrapErrorWithCaller wraps an error with context and caller information
func WrapErrorWithCaller(err error, ctx ErrorContext) error {
	if err == nil {
		return nil
	}
	ctx.WithCaller(1)
	return &ContextualError{
		Context: &ctx,
		Cause:   err,
	}
}

// NewContextualError creates a new contextual error
func NewContextualError(operation, resource, file string, cause error) error {
	ctx := NewErrorContext(operation, resource, file).WithCaller(1)
	return &ContextualError{
		Context: ctx,
		Cause:   cause,
	}
}

// ErrorRecovery defines strategies for error recovery
type ErrorRecovery interface {
	Recover(err error) error
	CanRecover(err error) bool
}

// DefaultErrorRecovery provides basic error recovery strategies
type DefaultErrorRecovery struct{}

// Recover attempts to recover from an error
func (der *DefaultErrorRecovery) Recover(err error) error {
	// Basic recovery logic - can be extended
	if IsRecoverable(err) {
		return nil // Recovery successful
	}
	return err // Cannot recover
}

// CanRecover determines if an error is recoverable
func (der *DefaultErrorRecovery) CanRecover(err error) bool {
	return IsRecoverable(err)
}

// IsRecoverable checks if an error is recoverable
func IsRecoverable(err error) bool {
	if err == nil {
		return true
	}

	// Check if it's a contextual error
	if ctxErr, ok := err.(*ContextualError); ok {
		return IsRecoverable(ctxErr.Cause)
	}

	// Add logic to determine recoverability based on error type
	// For now, consider most errors non-recoverable except specific cases
	switch {
	case strings.Contains(err.Error(), "timeout"):
		return true
	case strings.Contains(err.Error(), "temporary"):
		return true
	case strings.Contains(err.Error(), "connection refused"):
		return true
	default:
		return false
	}
}

// ErrorBuilder provides a fluent interface for building contextual errors
type ErrorBuilder struct {
	context *ErrorContext
}

// NewErrorBuilder creates a new error builder
func NewErrorBuilder() *ErrorBuilder {
	return &ErrorBuilder{
		context: &ErrorContext{
			Details: make(map[string]interface{}),
		},
	}
}

// Operation sets the operation
func (eb *ErrorBuilder) Operation(operation string) *ErrorBuilder {
	eb.context.Operation = operation
	return eb
}

// Resource sets the resource
func (eb *ErrorBuilder) Resource(resource string) *ErrorBuilder {
	eb.context.Resource = resource
	return eb
}

// File sets the file
func (eb *ErrorBuilder) File(file string) *ErrorBuilder {
	eb.context.File = file
	return eb
}

// Detail adds a detail
func (eb *ErrorBuilder) Detail(key string, value interface{}) *ErrorBuilder {
	eb.context.Details[key] = value
	return eb
}

// Wrap wraps an error with the built context
func (eb *ErrorBuilder) Wrap(err error) error {
	if err == nil {
		return nil
	}
	eb.context.WithCaller(1)
	return &ContextualError{
		Context: eb.context,
		Cause:   err,
	}
}

// New creates a new error with the built context
func (eb *ErrorBuilder) New(message string) error {
	eb.context.WithCaller(1)
	return &ContextualError{
		Context: eb.context,
		Cause:   fmt.Errorf("%s", message),
	}
}

// Convenience functions for common operations

// WrapFileError wraps a file operation error
func WrapFileError(err error, operation, filePath string) error {
	ctx := NewErrorContext(operation, "file", filePath).WithCaller(1)
	return WrapError(err, *ctx)
}

// WrapTemplateError wraps a template error
func WrapTemplateError(err error, operation, templateName string) error {
	ctx := NewErrorContext(operation, "template", templateName).
		WithDetail("template_name", templateName).
		WithCaller(1)
	return WrapError(err, *ctx)
}

// WrapValidationError wraps a validation error
func WrapValidationError(err error, field, value string) error {
	ctx := NewErrorContext("validation", field, "").
		WithDetail("value", value).
		WithCaller(1)
	return WrapError(err, *ctx)
}

// WrapDatabaseError wraps a database error
func WrapDatabaseError(err error, operation, table string) error {
	ctx := NewErrorContext(operation, "database", "").
		WithDetail("table", table).
		WithCaller(1)
	return WrapError(err, *ctx)
}

// WrapGenerationError wraps a generation error
func WrapGenerationError(err error, operation, resource string) error {
	ctx := NewErrorContext(operation, resource, "").WithCaller(1)
	return WrapError(err, *ctx)
}

package errors

import "fmt"

// GenerationError adds generation context to an underlying error.
type GenerationError struct {
	Operation string
	Resource  string
	File      string
	Cause     error
}

// Error returns the generation failure message.
func (e *GenerationError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("failed to %s %s in %s: %v", e.Operation, e.Resource, e.File, e.Cause)
	}
	return fmt.Sprintf("failed to %s %s: %v", e.Operation, e.Resource, e.Cause)
}

// Unwrap returns the underlying cause.
func (e *GenerationError) Unwrap() error {
	return e.Cause
}

// NewGeneratorError wraps an error that occurred while generating a resource.
func NewGeneratorError(operation, resource string, err error) error {
	return &GenerationError{
		Operation: operation,
		Resource:  resource,
		Cause:     err,
	}
}

// NewFileOperationError wraps an error that occurred while operating on a file.
func NewFileOperationError(path, operation string, err error) error {
	return &GenerationError{
		Operation: operation,
		Resource:  "file",
		File:      path,
		Cause:     err,
	}
}

// FileOperationError adds file path and operation context to an error.
type FileOperationError struct {
	Path      string
	Operation string
	Cause     error
}

// Error returns the file operation failure message.
func (e *FileOperationError) Error() string {
	return fmt.Sprintf("failed to %s file %s: %v", e.Operation, e.Path, e.Cause)
}

// Unwrap returns the underlying cause.
func (e *FileOperationError) Unwrap() error {
	return e.Cause
}

// NewSpecificFileOperationError wraps an error with file path and operation context.
func NewSpecificFileOperationError(path, operation string, err error) error {
	return &FileOperationError{
		Path:      path,
		Operation: operation,
		Cause:     err,
	}
}

// TemplateError adds template name and operation context to an error.
type TemplateError struct {
	TemplateName string
	Operation    string
	Cause        error
}

// Error returns the template failure message.
func (e *TemplateError) Error() string {
	return fmt.Sprintf("failed to %s template %s: %v", e.Operation, e.TemplateName, e.Cause)
}

// Unwrap returns the underlying cause.
func (e *TemplateError) Unwrap() error {
	return e.Cause
}

// NewTemplateError wraps an error from template processing.
func NewTemplateError(templateName, operation string, err error) error {
	return &TemplateError{
		TemplateName: templateName,
		Operation:    operation,
		Cause:        err,
	}
}

// ValidationError adds field, value, and reason context to a validation error.
type ValidationError struct {
	Field  string
	Value  string
	Reason string
	Cause  error
}

// Error returns the validation failure message.
func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation failed for %s '%s': %s (%v)", e.Field, e.Value, e.Reason, e.Cause)
	}
	return fmt.Sprintf("validation failed for %s '%s': %s", e.Field, e.Value, e.Reason)
}

// Unwrap returns the underlying cause.
func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a new validation error.
func NewValidationError(field, value, reason string, cause error) error {
	return &ValidationError{
		Field:  field,
		Value:  value,
		Reason: reason,
		Cause:  cause,
	}
}

// DatabaseError adds database operation and table context to an error.
type DatabaseError struct {
	Operation string
	Table     string
	Cause     error
}

// Error returns the database failure message.
func (e *DatabaseError) Error() string {
	if e.Table != "" {
		return fmt.Sprintf("database %s failed for table %s: %v", e.Operation, e.Table, e.Cause)
	}
	return fmt.Sprintf("database %s failed: %v", e.Operation, e.Cause)
}

// Unwrap returns the underlying cause.
func (e *DatabaseError) Unwrap() error {
	return e.Cause
}

// NewDatabaseError creates a new database error.
func NewDatabaseError(operation, table string, err error) error {
	return &DatabaseError{
		Operation: operation,
		Table:     table,
		Cause:     err,
	}
}

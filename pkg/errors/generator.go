package errors

import "fmt"

type GenerationError struct {
	Operation string
	Resource  string
	File      string
	Cause     error
}

func (e *GenerationError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("failed to %s %s in %s: %v", e.Operation, e.Resource, e.File, e.Cause)
	}
	return fmt.Sprintf("failed to %s %s: %v", e.Operation, e.Resource, e.Cause)
}

func (e *GenerationError) Unwrap() error {
	return e.Cause
}

func NewGeneratorError(operation, resource string, err error) error {
	return &GenerationError{
		Operation: operation,
		Resource:  resource,
		Cause:     err,
	}
}

func NewFileOperationError(path, operation string, err error) error {
	return &GenerationError{
		Operation: operation,
		Resource:  "file",
		File:      path,
		Cause:     err,
	}
}

type FileOperationError struct {
	Path      string
	Operation string
	Cause     error
}

func (e *FileOperationError) Error() string {
	return fmt.Sprintf("failed to %s file %s: %v", e.Operation, e.Path, e.Cause)
}

func (e *FileOperationError) Unwrap() error {
	return e.Cause
}

func NewSpecificFileOperationError(path, operation string, err error) error {
	return &FileOperationError{
		Path:      path,
		Operation: operation,
		Cause:     err,
	}
}

type TemplateError struct {
	TemplateName string
	Operation    string
	Cause        error
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("failed to %s template %s: %v", e.Operation, e.TemplateName, e.Cause)
}

func (e *TemplateError) Unwrap() error {
	return e.Cause
}

func NewTemplateError(templateName, operation string, err error) error {
	return &TemplateError{
		TemplateName: templateName,
		Operation:    operation,
		Cause:        err,
	}
}

type ValidationError struct {
	Field   string
	Value   string
	Reason  string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation failed for %s '%s': %s (%v)", e.Field, e.Value, e.Reason, e.Cause)
	}
	return fmt.Sprintf("validation failed for %s '%s': %s", e.Field, e.Value, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

func NewValidationError(field, value, reason string, cause error) error {
	return &ValidationError{
		Field:  field,
		Value:  value,
		Reason: reason,
		Cause:  cause,
	}
}

type DatabaseError struct {
	Operation string
	Table     string
	Cause     error
}

func (e *DatabaseError) Error() string {
	if e.Table != "" {
		return fmt.Sprintf("database %s failed for table %s: %v", e.Operation, e.Table, e.Cause)
	}
	return fmt.Sprintf("database %s failed: %v", e.Operation, e.Cause)
}

func (e *DatabaseError) Unwrap() error {
	return e.Cause
}

func NewDatabaseError(operation, table string, err error) error {
	return &DatabaseError{
		Operation: operation,
		Table:     table,
		Cause:     err,
	}
}
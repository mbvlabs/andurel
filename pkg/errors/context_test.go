package errors

import (
	"errors"
	"testing"
)

func TestErrorContext(t *testing.T) {
	t.Run("NewErrorContext", func(t *testing.T) {
		ctx := NewErrorContext("test", "resource", "file.txt")

		if ctx.Operation != "test" {
			t.Errorf("Expected operation 'test', got '%s'", ctx.Operation)
		}
		if ctx.Resource != "resource" {
			t.Errorf("Expected resource 'resource', got '%s'", ctx.Resource)
		}
		if ctx.File != "file.txt" {
			t.Errorf("Expected file 'file.txt', got '%s'", ctx.File)
		}
	})

	t.Run("WithDetail", func(t *testing.T) {
		ctx := NewErrorContext("test", "resource", "file.txt")
		ctx.WithDetail("key", "value")

		if ctx.Details["key"] != "value" {
			t.Errorf("Expected detail 'key' to be 'value', got '%v'", ctx.Details["key"])
		}
	})
}

func TestContextualError(t *testing.T) {
	t.Run("WrapError", func(t *testing.T) {
		originalErr := errors.New("original error")
		ctx := NewErrorContext("operation", "resource", "file.txt")

		wrappedErr := WrapError(originalErr, *ctx)

		if wrappedErr == nil {
			t.Fatal("Expected non-nil error")
		}

		if !errors.Is(wrappedErr, originalErr) {
			t.Error("Wrapped error should wrap the original error")
		}

		errorStr := wrappedErr.Error()
		expected := "operation: operation, resource: resource, file: file.txt: original error"
		if errorStr != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, errorStr)
		}
	})

	t.Run("WrapErrorWithNil", func(t *testing.T) {
		ctx := NewErrorContext("operation", "resource", "file.txt")
		wrappedErr := WrapError(nil, *ctx)

		if wrappedErr != nil {
			t.Error("Wrapping nil error should return nil")
		}
	})
}

func TestErrorBuilder(t *testing.T) {
	t.Run("BuildAndWrap", func(t *testing.T) {
		originalErr := errors.New("test error")

		wrappedErr := NewErrorBuilder().
			Operation("test-op").
			Resource("test-resource").
			File("test.txt").
			Detail("extra", "info").
			Wrap(originalErr)

		if wrappedErr == nil {
			t.Fatal("Expected non-nil error")
		}

		errorStr := wrappedErr.Error()
		if !contains(errorStr, "operation: test-op") {
			t.Error("Error should contain operation")
		}
		if !contains(errorStr, "resource: test-resource") {
			t.Error("Error should contain resource")
		}
		if !contains(errorStr, "file: test.txt") {
			t.Error("Error should contain file")
		}
		if !contains(errorStr, "extra: info") {
			t.Error("Error should contain details")
		}
	})

	t.Run("BuildAndNew", func(t *testing.T) {
		newErr := NewErrorBuilder().
			Operation("create").
			Resource("user").
			New("failed to create user")

		if newErr == nil {
			t.Fatal("Expected non-nil error")
		}

		errorStr := newErr.Error()
		if !contains(errorStr, "operation: create") {
			t.Error("Error should contain operation")
		}
		if !contains(errorStr, "resource: user") {
			t.Error("Error should contain resource")
		}
		if !contains(errorStr, "failed to create user") {
			t.Error("Error should contain message")
		}
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("WrapFileError", func(t *testing.T) {
		originalErr := errors.New("file not found")
		wrappedErr := WrapFileError(originalErr, "read", "/path/to/file.txt")

		if wrappedErr == nil {
			t.Fatal("Expected non-nil error")
		}

		errorStr := wrappedErr.Error()
		if !contains(errorStr, "operation: read") {
			t.Error("Error should contain operation")
		}
		if !contains(errorStr, "resource: file") {
			t.Error("Error should contain resource")
		}
		if !contains(errorStr, "file: /path/to/file.txt") {
			t.Error("Error should contain file")
		}
	})

	t.Run("WrapTemplateError", func(t *testing.T) {
		originalErr := errors.New("template syntax error")
		wrappedErr := WrapTemplateError(originalErr, "parse", "template.tmpl")

		if wrappedErr == nil {
			t.Fatal("Expected non-nil error")
		}

		errorStr := wrappedErr.Error()
		if !contains(errorStr, "operation: parse") {
			t.Error("Error should contain operation")
		}
		if !contains(errorStr, "resource: template") {
			t.Error("Error should contain resource")
		}
		if !contains(errorStr, "template_name: template.tmpl") {
			t.Error("Error should contain template name")
		}
	})
}

func TestErrorRecovery(t *testing.T) {
	recovery := &DefaultErrorRecovery{}

	t.Run("RecoverableError", func(t *testing.T) {
		err := errors.New("operation timeout")

		if !recovery.CanRecover(err) {
			t.Error("Timeout error should be recoverable")
		}

		recoveredErr := recovery.Recover(err)
		if recoveredErr != nil {
			t.Error("Recovery should return nil for recoverable error")
		}
	})

	t.Run("NonRecoverableError", func(t *testing.T) {
		err := errors.New("syntax error")

		if recovery.CanRecover(err) {
			t.Error("Syntax error should not be recoverable")
		}

		recoveredErr := recovery.Recover(err)
		if recoveredErr == nil {
			t.Error("Recovery should return error for non-recoverable error")
		}
		if recoveredErr != err {
			t.Error("Recovery should return original error for non-recoverable error")
		}
	})
}

func TestIsRecoverable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"NilError", nil, true},
		{"TimeoutError", errors.New("operation timeout"), true},
		{"TemporaryError", errors.New("temporary failure"), true},
		{"ConnectionRefused", errors.New("connection refused"), true},
		{"SyntaxError", errors.New("syntax error"), false},
		{"ValidationError", errors.New("validation failed"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverable(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

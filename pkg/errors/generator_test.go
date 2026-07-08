package errors

import (
	stderrors "errors"
	"strings"
	"testing"
)

func TestGenerationError(t *testing.T) {
	cause := stderrors.New("disk full")

	err := NewGeneratorError("generate", "controller", cause)
	if !stderrors.Is(err, cause) {
		t.Fatalf("NewGeneratorError should wrap cause")
	}
	if got, want := err.Error(), "failed to generate controller: disk full"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	fileErr := NewFileOperationError("controllers/users.go", "write", cause)
	if !stderrors.Is(fileErr, cause) {
		t.Fatalf("NewFileOperationError should wrap cause")
	}
	if got, want := fileErr.Error(), "failed to write file in controllers/users.go: disk full"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestSpecificFileOperationError(t *testing.T) {
	cause := stderrors.New("permission denied")

	err := NewSpecificFileOperationError("views/index.templ", "read", cause)
	if !stderrors.Is(err, cause) {
		t.Fatalf("NewSpecificFileOperationError should wrap cause")
	}
	if got, want := err.Error(), "failed to read file views/index.templ: permission denied"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestTemplateValidationAndDatabaseErrors(t *testing.T) {
	cause := stderrors.New("bad input")

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "template",
			err:  NewTemplateError("model.go.tmpl", "execute", cause),
			want: "failed to execute template model.go.tmpl: bad input",
		},
		{
			name: "validation without cause",
			err:  NewValidationError("resource", "users", "must be singular", nil),
			want: "validation failed for resource 'users': must be singular",
		},
		{
			name: "validation with cause",
			err:  NewValidationError("resource", "users", "must be singular", cause),
			want: "validation failed for resource 'users': must be singular (bad input)",
		},
		{
			name: "database table",
			err:  NewDatabaseError("query", "users", cause),
			want: "database query failed for table users: bad input",
		},
		{
			name: "database no table",
			err:  NewDatabaseError("connect", "", cause),
			want: "database connect failed: bad input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
			if strings.Contains(tt.name, "without cause") {
				if stderrors.Unwrap(tt.err) != nil {
					t.Fatalf("expected nil unwrap for %s", tt.name)
				}
				return
			}
			if !stderrors.Is(tt.err, cause) {
				t.Fatalf("%s should wrap cause", tt.name)
			}
		})
	}
}

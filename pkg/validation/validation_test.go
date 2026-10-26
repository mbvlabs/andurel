package validation

import (
	"errors"
	"fmt"
	"testing"
)

func TestJoinDropsNilErrors(t *testing.T) {
	if err := Join(); err != nil {
		t.Fatalf("Join() = %v, want nil", err)
	}

	if err := Join(nil, nil); err != nil {
		t.Fatalf("Join(nil, nil) = %v, want nil", err)
	}
}

func TestJoinFlattensValidationErrors(t *testing.T) {
	first := ValidationErrors{{Field: "email", Code: "required", Message: "is required"}}
	second := ValidationErrors{{Field: "password", Code: "min", Message: "too short"}}

	err := Join(first, nil, second)

	got, ok := As(err)
	if !ok {
		t.Fatalf("Join() = %v, want a validation error", err)
	}
	if got.Len() != 2 {
		t.Fatalf("Join() errors = %d, want 2", got.Len())
	}
	if got[0].Field != "email" || got[1].Field != "password" {
		t.Fatalf("Join() fields = %q, %q", got[0].Field, got[1].Field)
	}
}

func TestJoinFlattensNestedJoins(t *testing.T) {
	first := ValidationErrors{{Field: "email", Code: "required", Message: "is required"}}
	second := ValidationErrors{{Field: "password", Code: "min", Message: "too short"}}
	third := ValidationErrors{{Field: "code", Code: "invalid", Message: "invalid code"}}

	err := Join(errors.Join(first, second), fmt.Errorf("wrapped: %w", third))

	got, ok := As(err)
	if !ok {
		t.Fatalf("Join() = %v, want a validation error", err)
	}
	if got.Len() != 3 {
		t.Fatalf("Join() errors = %d, want 3", got.Len())
	}
}

func TestJoinKeepsNonValidationErrors(t *testing.T) {
	validationErr := ValidationErrors{{Field: "email", Code: "required", Message: "is required"}}
	otherErr := errors.New("database unavailable")

	err := Join(validationErr, otherErr)

	got, ok := As(err)
	if !ok {
		t.Fatalf("Join() = %v, want a validation error", err)
	}
	if got.Len() != 1 {
		t.Fatalf("Join() errors = %d, want 1", got.Len())
	}
	if !errors.Is(err, otherErr) {
		t.Fatalf("Join() = %v, want it to wrap %v", err, otherErr)
	}
}

func TestJoinPreservesWrappedErrorTrees(t *testing.T) {
	first := Field("email", "required", "is required")
	second := Field("password", "required", "is required")
	sentinel := errors.New("domain sentinel")
	wrapped := fmt.Errorf("service: %w", errors.Join(sentinel, first, second))

	err := Join(wrapped)

	fields, ok := As(err)
	if !ok || fields.Len() != 2 {
		t.Fatalf("Join() fields = %v, want both field errors", fields)
	}
	if fields[0].Field != "email" || fields[1].Field != "password" {
		t.Fatalf("Join() fields = %v, want email then password", fields)
	}
	if !errors.Is(err, sentinel) || !errors.Is(err, wrapped) {
		t.Fatalf("Join() = %v, want the original error tree preserved", err)
	}
}

func TestJoinOnlyNonValidationErrors(t *testing.T) {
	first := errors.New("first")
	second := errors.New("second")

	err := Join(first, nil, second)

	if _, ok := As(err); ok {
		t.Fatalf("Join() = %v, want no validation error", err)
	}
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("Join() = %v, want it to wrap both errors", err)
	}
}

func TestEmail(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "user@example.com"},
		{name: "empty", value: ""},
		{name: "malformed", value: "not-an-email", wantErr: true},
		{name: "display name", value: "User <user@example.com>", wantErr: true},
		{name: "trailing space", value: "user@example.com ", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := NewBuilder()
			b.Email("email", test.value)

			if got := b.Err() != nil; got != test.wantErr {
				t.Fatalf("Email(%q) error = %v, wantErr %v", test.value, b.Err(), test.wantErr)
			}
			rules := b.Rules()["email"]
			if len(rules) != 1 || rules[0].Code != "email" {
				t.Fatalf("Email(%q) rules = %#v", test.value, rules)
			}
		})
	}
}

func TestEmailCustomMessage(t *testing.T) {
	b := NewBuilder()
	b.Email("email", "not-an-email", "Enter a valid email")

	err, ok := As(b.Err())
	if !ok {
		t.Fatal("Email() returned no validation error")
	}
	if err[0].Message != "Enter a valid email" {
		t.Fatalf("Email() message = %q", err[0].Message)
	}
}

func TestField(t *testing.T) {
	err := Field("code", "invalid", "Invalid or expired code")

	got, ok := As(err)
	if !ok {
		t.Fatalf("Field() = %v, want a validation error", err)
	}
	if got.Len() != 1 {
		t.Fatalf("Field() errors = %d, want 1", got.Len())
	}
	if got[0].Field != "code" || got[0].Code != "invalid" ||
		got[0].Message != "Invalid or expired code" {
		t.Fatalf("Field() = %#v", got[0])
	}
}

func TestFieldNormalisesEmptyField(t *testing.T) {
	err := Field("", "invalid", "Invalid")

	got, ok := As(err)
	if !ok {
		t.Fatalf("Field() = %v, want a validation error", err)
	}
	if got[0].Field != UnknownField {
		t.Fatalf("Field() field = %q, want %q", got[0].Field, UnknownField)
	}
}

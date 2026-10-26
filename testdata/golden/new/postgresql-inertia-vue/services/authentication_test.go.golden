package services

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/mbvlabs/andurel/pkg/validation"
)

func TestVerifyEmailRejectsMissingPendingUser(t *testing.T) {
	identity := Identity{}

	_, err := identity.VerifyEmail(context.Background(), VerifyEmailData{
		UserID: uuid.Nil(),
		Code:   "ABC123",
	})
	if !errors.Is(err, ErrInvalidVerificationCode) {
		t.Fatalf("VerifyEmail error = %v, want %v", err, ErrInvalidVerificationCode)
	}

	validationErrors, ok := validation.As(err)
	if !ok {
		t.Fatalf("VerifyEmail error = %v, want a validation error", err)
	}
	if got := validationErrors.ToMap()["code"]; got != invalidVerificationCodeMessage {
		t.Fatalf("code error = %q, want %q", got, invalidVerificationCodeMessage)
	}
}

func TestVerificationAttemptsAreBoundedByTheLimit(t *testing.T) {
	if !verificationAttemptAllowed(maxVerificationAttempts) {
		t.Fatal("the final allowed attempt must be checked")
	}
	if verificationAttemptAllowed(maxVerificationAttempts + 1) {
		t.Fatal("attempts past the limit must not be checked")
	}

	if verificationLocked(maxVerificationAttempts - 1) {
		t.Fatal("a failure below the limit must not lock out")
	}
	if !verificationLocked(maxVerificationAttempts) {
		t.Fatal("the failure that reaches the limit must lock out")
	}
}

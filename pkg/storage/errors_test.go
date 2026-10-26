package storage

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRowReturnsValue(t *testing.T) {
	got, err := Row("user", nil)
	if err != nil {
		t.Fatalf("Row: %v", err)
	}
	if got != "user" {
		t.Fatalf("Row = %q, want %q", got, "user")
	}
}

func TestRowMapsNoRowsToNotFound(t *testing.T) {
	got, err := Row("user", pgx.ErrNoRows)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Row error = %v, want %v", err, ErrNotFound)
	}
	if got != "" {
		t.Fatalf("Row = %q, want the zero value", got)
	}
}

func TestRowKeepsOtherErrors(t *testing.T) {
	other := errors.New("database unavailable")

	got, err := Row("user", other)
	if !errors.Is(err, other) {
		t.Fatalf("Row error = %v, want %v", err, other)
	}
	if got != "" {
		t.Fatalf("Row = %q, want the zero value", got)
	}
}

func TestConflictWrapsMatchingUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "users_email_key",
	}

	err := Conflict(pgErr, "users_email_key")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Conflict error = %v, want %v", err, ErrConflict)
	}
	if !errors.Is(err, pgErr) {
		t.Fatalf("Conflict error = %v, want it to wrap the pg error", err)
	}
}

func TestConflictKeepsOtherErrors(t *testing.T) {
	uniqueViolation := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "users_email_key",
	}
	tests := []struct {
		name       string
		err        error
		constraint string
	}{
		{name: "different constraint", err: uniqueViolation, constraint: "tokens_hash_key"},
		{name: "different code", err: &pgconn.PgError{Code: "23503", ConstraintName: "users_email_key"}, constraint: "users_email_key"},
		{name: "not a pg error", err: errors.New("database unavailable"), constraint: "users_email_key"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Conflict(test.err, test.constraint)
			if errors.Is(err, ErrConflict) {
				t.Fatalf("Conflict error = %v, want no ErrConflict", err)
			}
			if !errors.Is(err, test.err) {
				t.Fatalf("Conflict error = %v, want %v unchanged", err, test.err)
			}
		})
	}
}

func TestConflictSeesWrappedUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "users_email_key",
	}

	err := Conflict(fmt.Errorf("create user: %w", pgErr), "users_email_key")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Conflict error = %v, want %v", err, ErrConflict)
	}
}

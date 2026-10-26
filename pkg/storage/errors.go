package storage

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound reports that a query expected a row but found none.
	ErrNotFound = errors.New("storage: not found")
	// ErrConflict reports that a write violated a unique constraint.
	ErrConflict = errors.New("storage: conflict")
)

// Row maps pgx.ErrNoRows to ErrNotFound and returns any other error unchanged.
func Row[T any](v T, err error) (T, error) {
	if err == nil {
		return v, nil
	}

	var zero T
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, ErrNotFound
	}

	return zero, err
}

// Conflict wraps ErrConflict when err is a unique violation of constraint.
// Unrelated errors are returned unchanged.
func Conflict(err error, constraint string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.ConstraintName == constraint {
		return errors.Join(ErrConflict, err)
	}

	return err
}

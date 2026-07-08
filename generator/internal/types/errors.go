package types

import "errors"

var (
	// ErrNoTypeMapperFound is returned when no type mapper found.
	ErrNoTypeMapperFound = errors.New("no type mapper found for database type")
	// ErrUnsupportedType is returned when unsupported type.
	ErrUnsupportedType = errors.New("unsupported database type")
)

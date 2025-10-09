package types

import "errors"

var (
	ErrNoTypeMapperFound = errors.New("no type mapper found for database type")
	ErrUnsupportedType   = errors.New("unsupported database type")
)

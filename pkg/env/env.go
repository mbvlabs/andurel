// Package env loads .env files and reads typed values from the process
// environment.
package env

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Reader reads typed values from a lookup function and collects conversion
// errors until Err is called.
type Reader struct {
	lookup func(string) (string, bool)
	errs   []error
}

// New returns a Reader backed by the process environment.
func New() *Reader {
	return NewWithLookup(os.LookupEnv)
}

// NewWithLookup returns a Reader backed by lookup.
func NewWithLookup(lookup func(string) (string, bool)) *Reader {
	return &Reader{lookup: lookup}
}

// String returns the value for key, or fallback when it is not set.
func (e *Reader) String(key, fallback string) string {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}

	return value
}

// RequiredString returns the value for key and records an error when it is
// missing or blank.
func (e *Reader) RequiredString(key string) string {
	value, ok := e.lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		e.errs = append(e.errs, fmt.Errorf("%s is required", key))
	}

	return value
}

// Int returns key as an int, or fallback when it is not set.
func (e *Reader) Int(key string, fallback int) int {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		e.add(key, "integer", value, err)
		return fallback
	}

	return parsed
}

// Int32 returns key as an int32, or fallback when it is not set.
func (e *Reader) Int32(key string, fallback int32) int32 {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		e.add(key, "32-bit integer", value, err)
		return fallback
	}

	return int32(parsed)
}

// Int64 returns key as an int64, or fallback when it is not set.
func (e *Reader) Int64(key string, fallback int64) int64 {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		e.add(key, "64-bit integer", value, err)
		return fallback
	}

	return parsed
}

// Bool returns key as a bool, or fallback when it is not set.
func (e *Reader) Bool(key string, fallback bool) bool {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		e.add(key, "boolean", value, err)
		return fallback
	}

	return parsed
}

// Float64 returns key as a float64, or fallback when it is not set.
func (e *Reader) Float64(key string, fallback float64) float64 {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		e.add(key, "number", value, err)
		return fallback
	}

	return parsed
}

// Duration returns key as a time.Duration, or fallback when it is not set.
func (e *Reader) Duration(key string, fallback time.Duration) time.Duration {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		e.add(key, "duration", value, err)
		return fallback
	}

	return parsed
}

// Strings returns key as a comma-separated list, or a copy of fallback when it
// is not set.
func (e *Reader) Strings(key string, fallback []string) []string {
	value, ok := e.lookup(key)
	if !ok {
		return slices.Clone(fallback)
	}
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}

	return out
}

// Err returns the conversion and required-value errors collected so far.
func (e *Reader) Err() error {
	return errors.Join(e.errs...)
}

func (e *Reader) add(key, expected, value string, err error) {
	e.errs = append(e.errs, fmt.Errorf(
		"%s must be a valid %s (got %q): %w",
		key,
		expected,
		value,
		err,
	))
}

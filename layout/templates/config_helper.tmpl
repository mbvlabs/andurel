package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type environment struct {
	lookup func(string) (string, bool)
	errs   []error
}

func newEnvironment() *environment {
	return newEnvironmentWithLookup(os.LookupEnv)
}

func newEnvironmentWithLookup(lookup func(string) (string, bool)) *environment {
	return &environment{lookup: lookup}
}

// LoadEnvironment loads a dotenv file before configuration constructors run.
// A missing default .env file is allowed because deployed applications commonly
// receive their environment directly from the operating system.
func LoadEnvironment(filenames ...string) error {
	err := godotenv.Load(filenames...)
	if err == nil || (len(filenames) == 0 && os.IsNotExist(err)) {
		return nil
	}

	return fmt.Errorf("config: load environment: %w", err)
}

func (e *environment) String(key, fallback string) string {
	value, ok := e.lookup(key)
	if !ok {
		return fallback
	}

	return value
}

func (e *environment) RequiredString(key string) string {
	value, ok := e.lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		e.errs = append(e.errs, fmt.Errorf("%s is required", key))
	}

	return value
}

func (e *environment) Int(key string, fallback int) int {
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

func (e *environment) Int32(key string, fallback int32) int32 {
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

func (e *environment) Int64(key string, fallback int64) int64 {
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

func (e *environment) Bool(key string, fallback bool) bool {
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

func (e *environment) Float64(key string, fallback float64) float64 {
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

func (e *environment) Duration(key string, fallback time.Duration) time.Duration {
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

func (e *environment) Strings(key string, fallback []string) []string {
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

func (e *environment) Err() error {
	return errors.Join(e.errs...)
}

func (e *environment) add(key, expected, value string, err error) {
	e.errs = append(e.errs, fmt.Errorf(
		"%s must be a valid %s (got %q): %w",
		key,
		expected,
		value,
		err,
	))
}

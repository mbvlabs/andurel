package env

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	reader := testReader(map[string]string{"KEY": "value"})

	if got := reader.String("KEY", "fallback"); got != "value" {
		t.Fatalf("String(KEY) = %q, want %q", got, "value")
	}
	if got := reader.String("MISSING", "fallback"); got != "fallback" {
		t.Fatalf("String(MISSING) = %q, want %q", got, "fallback")
	}
}

func TestRequiredString(t *testing.T) {
	reader := testReader(map[string]string{"KEY": "value", "BLANK": "   "})

	if got := reader.RequiredString("KEY"); got != "value" {
		t.Fatalf("RequiredString(KEY) = %q, want %q", got, "value")
	}
	if err := reader.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}

	reader.RequiredString("MISSING")
	reader.RequiredString("BLANK")

	err := reader.Err()
	if err == nil {
		t.Fatal("Err() = nil, want required errors")
	}
	if !strings.Contains(err.Error(), "MISSING is required") ||
		!strings.Contains(err.Error(), "BLANK is required") {
		t.Fatalf("Err() = %q, want both required errors", err)
	}
}

func TestInt(t *testing.T) {
	reader := testReader(map[string]string{"PORT": "8080", "BAD": "abc"})

	if got := reader.Int("PORT", 1); got != 8080 {
		t.Fatalf("Int(PORT) = %d, want 8080", got)
	}
	if got := reader.Int("MISSING", 1); got != 1 {
		t.Fatalf("Int(MISSING) = %d, want 1", got)
	}
	if got := reader.Int("BAD", 2); got != 2 {
		t.Fatalf("Int(BAD) = %d, want 2", got)
	}

	want := `BAD must be a valid integer (got "abc"): strconv.Atoi: parsing "abc": invalid syntax`
	if err := reader.Err(); err == nil || err.Error() != want {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
}

func TestInt32(t *testing.T) {
	reader := testReader(map[string]string{"LOCK": "12", "BAD": "abc"})

	if got := reader.Int32("LOCK", 1); got != 12 {
		t.Fatalf("Int32(LOCK) = %d, want 12", got)
	}
	if got := reader.Int32("MISSING", 1); got != 1 {
		t.Fatalf("Int32(MISSING) = %d, want 1", got)
	}
	if got := reader.Int32("BAD", 2); got != 2 {
		t.Fatalf("Int32(BAD) = %d, want 2", got)
	}

	want := `BAD must be a valid 32-bit integer (got "abc"): strconv.ParseInt: parsing "abc": invalid syntax`
	if err := reader.Err(); err == nil || err.Error() != want {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
}

func TestInt64(t *testing.T) {
	reader := testReader(map[string]string{"BYTES": "2097152", "BAD": "abc"})

	if got := reader.Int64("BYTES", 1); got != 2097152 {
		t.Fatalf("Int64(BYTES) = %d, want 2097152", got)
	}
	if got := reader.Int64("MISSING", 1); got != 1 {
		t.Fatalf("Int64(MISSING) = %d, want 1", got)
	}
	if got := reader.Int64("BAD", 2); got != 2 {
		t.Fatalf("Int64(BAD) = %d, want 2", got)
	}

	want := `BAD must be a valid 64-bit integer (got "abc"): strconv.ParseInt: parsing "abc": invalid syntax`
	if err := reader.Err(); err == nil || err.Error() != want {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
}

func TestBool(t *testing.T) {
	reader := testReader(map[string]string{"ENABLED": "true", "BAD": "abc"})

	if !reader.Bool("ENABLED", false) {
		t.Fatal("Bool(ENABLED) = false, want true")
	}
	if !reader.Bool("MISSING", true) {
		t.Fatal("Bool(MISSING) = false, want the fallback true")
	}
	if reader.Bool("BAD", true) != true {
		t.Fatal("Bool(BAD) = false, want the fallback true")
	}

	want := `BAD must be a valid boolean (got "abc"): strconv.ParseBool: parsing "abc": invalid syntax`
	if err := reader.Err(); err == nil || err.Error() != want {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
}

func TestFloat64(t *testing.T) {
	reader := testReader(map[string]string{"RATE": "0.5", "BAD": "abc"})

	if got := reader.Float64("RATE", 1); got != 0.5 {
		t.Fatalf("Float64(RATE) = %v, want 0.5", got)
	}
	if got := reader.Float64("MISSING", 1); got != 1 {
		t.Fatalf("Float64(MISSING) = %v, want 1", got)
	}
	if got := reader.Float64("BAD", 2); got != 2 {
		t.Fatalf("Float64(BAD) = %v, want 2", got)
	}

	want := `BAD must be a valid number (got "abc"): strconv.ParseFloat: parsing "abc": invalid syntax`
	if err := reader.Err(); err == nil || err.Error() != want {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
}

func TestDuration(t *testing.T) {
	reader := testReader(map[string]string{"TIMEOUT": "2s", "BAD": "abc"})

	if got := reader.Duration("TIMEOUT", time.Second); got != 2*time.Second {
		t.Fatalf("Duration(TIMEOUT) = %v, want 2s", got)
	}
	if got := reader.Duration("MISSING", time.Second); got != time.Second {
		t.Fatalf("Duration(MISSING) = %v, want 1s", got)
	}
	if got := reader.Duration("BAD", 2*time.Second); got != 2*time.Second {
		t.Fatalf("Duration(BAD) = %v, want 2s", got)
	}

	want := `BAD must be a valid duration (got "abc"): time: invalid duration "abc"`
	if err := reader.Err(); err == nil || err.Error() != want {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
}

func TestStrings(t *testing.T) {
	reader := testReader(map[string]string{
		"LIST":  "first, second, , third,",
		"EMPTY": "",
	})

	if got := reader.Strings("LIST", nil); !slices.Equal(got, []string{"first", "second", "third"}) {
		t.Fatalf("Strings(LIST) = %#v", got)
	}
	if got := reader.Strings("EMPTY", []string{"fallback"}); got == nil || len(got) != 0 {
		t.Fatalf("Strings(EMPTY) = %#v, want an empty non-nil list", got)
	}

	fallback := []string{"one", "two"}
	got := reader.Strings("MISSING", fallback)
	if !slices.Equal(got, fallback) {
		t.Fatalf("Strings(MISSING) = %#v, want %#v", got, fallback)
	}
	got[0] = "changed"
	if fallback[0] != "one" {
		t.Fatal("Strings(MISSING) modified the fallback slice")
	}
}

func TestNewReadsProcessEnvironment(t *testing.T) {
	t.Setenv("ANDUREL_ENV_TEST_READER", "value")

	if got := New().String("ANDUREL_ENV_TEST_READER", "fallback"); got != "value" {
		t.Fatalf("String() = %q, want %q", got, "value")
	}
}

func testReader(values map[string]string) *Reader {
	return NewWithLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
}

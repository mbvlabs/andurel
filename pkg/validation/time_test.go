package validation

import (
	"testing"
	"time"
)

func TestTimeBetween(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	const (
		min = time.Hour
		max = 24 * time.Hour
	)

	pointer := now.Add(2 * time.Hour)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "inside window", value: now.Add(2 * time.Hour)},
		{name: "at minimum", value: now.Add(min)},
		{name: "at maximum", value: now.Add(max)},
		{name: "before minimum", value: now.Add(min - time.Nanosecond), wantErr: true},
		{name: "after maximum", value: now.Add(max + time.Nanosecond), wantErr: true},
		{name: "pointer inside window", value: &pointer},
		{name: "missing time", value: time.Time{}},
		{name: "missing pointer", value: (*time.Time)(nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := NewBuilder()
			b.TimeBetween("expires_at", test.value, now, min, max)

			if got := b.Err() != nil; got != test.wantErr {
				t.Fatalf("TimeBetween(%v) error = %v, wantErr %v", test.value, b.Err(), test.wantErr)
			}

			rules := b.Rules()["expires_at"]
			if len(rules) != 1 || rules[0].Code != "time_between" {
				t.Fatalf("TimeBetween(%v) rules = %#v", test.value, rules)
			}
		})
	}
}

func TestTimeBetweenUsesExplicitNow(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(2 * time.Hour)

	passing := NewBuilder()
	passing.TimeBetween("expires_at", expiresAt, now, time.Hour, 24*time.Hour)
	if passing.Err() != nil {
		t.Fatalf("TimeBetween with explicit now = %v, want nil", passing.Err())
	}

	late := NewBuilder()
	late.TimeBetween("expires_at", expiresAt, now.Add(2*time.Hour), time.Hour, 24*time.Hour)

	err, ok := As(late.Err())
	if !ok || err.Len() != 1 {
		t.Fatalf("TimeBetween with later now = %v, want one error", late.Err())
	}
	if err[0].Code != "time_between" || err[0].Field != "expires_at" {
		t.Fatalf("TimeBetween error = %#v", err[0])
	}
}

func TestTimeBetweenCustomMessage(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)

	b := NewBuilder()
	b.TimeBetween(
		"expires_at",
		now.Add(time.Hour-time.Nanosecond),
		now,
		time.Hour,
		24*time.Hour,
		"Choose an expiry between one hour and one day",
	)

	err, ok := As(b.Err())
	if !ok || err.Len() != 1 {
		t.Fatalf("TimeBetween = %v, want one error", b.Err())
	}
	if err[0].Message != "Choose an expiry between one hour and one day" {
		t.Fatalf("TimeBetween message = %q", err[0].Message)
	}
}

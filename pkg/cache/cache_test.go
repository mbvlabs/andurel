package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryPutGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := New(Config{Driver: "memory"})
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}

	if err := store.Put(ctx, "greeting", []byte(`"hello"`), time.Minute); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, ok, err := Get[string](ctx, store, "greeting")
	if err != nil || !ok {
		t.Fatalf("get = %v, %v, %v; want hello, true, nil", got, ok, err)
	}
	if got != "hello" {
		t.Fatalf("get = %q, want %q", got, "hello")
	}
}

func TestMemoryExpiry(t *testing.T) {
	ctx := context.Background()
	store, err := New(Config{Driver: "memory"})
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}

	if err := store.Put(ctx, "short", []byte(`1`), 20*time.Millisecond); err != nil {
		t.Fatalf("put: %v", err)
	}

	if _, ok, _ := store.Get(ctx, "short"); !ok {
		t.Fatal("expected key before expiry")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, ok, err := store.Get(ctx, "short")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("key survived past its ttl")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRememberLoadsOnce(t *testing.T) {
	ctx := context.Background()
	store, err := New(Config{Driver: "memory"})
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}

	calls := 0
	loader := func(context.Context) (string, error) {
		calls++

		return "computed", nil
	}

	for range 3 {
		got, err := Remember(ctx, store, "once", time.Minute, loader)
		if err != nil || got != "computed" {
			t.Fatalf("remember = %q, %v; want computed, nil", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("loader ran %d times, want 1", calls)
	}
}

func TestRememberLoaderError(t *testing.T) {
	ctx := context.Background()
	store, err := New(Config{Driver: "memory"})
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}

	boom := errors.New("boom")
	_, err = Remember(ctx, store, "bad", time.Minute, func(context.Context) (string, error) {
		return "", boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("remember err = %v, want %v", err, boom)
	}

	if _, ok, _ := store.Get(ctx, "bad"); ok {
		t.Fatal("loader error must not be cached")
	}
}

func TestIncrementWindow(t *testing.T) {
	ctx := context.Background()
	store, err := New(Config{Driver: "memory"})
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}

	for want := int64(1); want <= 3; want++ {
		got, err := store.Increment(ctx, "ip:1.2.3.4", 50*time.Millisecond)
		if err != nil || got != want {
			t.Fatalf("increment = %d, %v; want %d, nil", got, err, want)
		}
	}
}

func TestUnknownDriver(t *testing.T) {
	if _, err := New(Config{Driver: "memcached"}); err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

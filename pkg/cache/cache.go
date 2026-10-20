// Package cache provides a unified, Laravel-inspired cache for Andurel
// applications with swappable drivers.
//
// A Store is keyed by string and holds JSON-encoded values, so the memory and
// Redis drivers behave identically: whatever Remember writes, Get reads back
// the same way on either driver. Typed access goes through the free functions
// Get and Remember; the Store interface itself stays byte-oriented because Go
// interfaces cannot carry type parameters.
//
// Drivers: "memory" (otter, in-process) and "redis" (shared across replicas,
// required for rate limiting behind more than one instance). Pick one in
// Config and build it once in internal/runtime; handlers and middleware take
// a Store, never a driver.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Store is the driver contract. All methods take a context and namespace keys
// internally, so callers pass bare keys like "etag:/assets/app.css".
type Store interface {
	// Get returns the stored value and true, or false when the key is
	// missing or expired. Corrupt values are treated as missing.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Put stores value for ttl. A ttl <= 0 means no expiry.
	Put(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Forget deletes key. Forgetting a missing key is not an error.
	Forget(ctx context.Context, key string) error
	// Flush deletes every key in the store's namespace. On Redis this
	// scans only namespaced keys; it never runs FLUSHDB.
	Flush(ctx context.Context) error
	// Increment counts a hit inside a fixed window and returns the new
	// count. It is the rate-limiting primitive: allow the request while
	// count <= limit. The window starts on the first hit.
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)
	// Close releases the driver's resources, such as the Redis connection
	// pool. The store must not be used afterwards.
	Close() error
}

// Get decodes the value at key into T. It returns false when the key is
// missing, expired, or held by the other driver in an incompatible shape.
func Get[T any](ctx context.Context, s Store, key string) (T, bool, error) {
	var zero T

	raw, ok, err := s.Get(ctx, key)
	if err != nil || !ok {
		return zero, false, err
	}

	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, false, nil
	}

	return value, true, nil
}

// flights deduplicates concurrent Remember loaders per store+key so a cold
// key loads once even under a thundering herd.
var flights singleflight

// Remember returns the cached value at key, loading, storing, and returning
// it when missing. Concurrent callers for the same store+key share one loader
// invocation. Loader errors are returned without caching anything.
func Remember[T any](
	ctx context.Context,
	s Store,
	key string,
	ttl time.Duration,
	loader func(context.Context) (T, error),
) (T, error) {
	if value, ok, err := Get[T](ctx, s, key); err != nil {
		var zero T

		return zero, err
	} else if ok {
		return value, nil
	}

	loaded, err, _ := flights.Do(storeKey(s, key), func() (any, error) {
		return loader(ctx)
	})
	if err != nil {
		var zero T

		return zero, err
	}

	value, ok := loaded.(T)
	if !ok {
		var zero T

		return zero, fmt.Errorf("cache: loader returned %T, want %T", loaded, zero)
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return value, fmt.Errorf("cache: encode %s: %w", key, err)
	}

	if err := s.Put(ctx, key, raw, ttl); err != nil {
		return value, err
	}

	return value, nil
}

// storeKey identifies one Remember flight.
func storeKey(s Store, key string) string {
	return fmt.Sprintf("%p/%s", s, key)
}

// singleflight is a minimal duplicate-suppression group: concurrent Do calls
// with the same key share one invocation of fn.
type singleflight struct {
	mu sync.Mutex
	m  map[string]*flight
}

type flight struct {
	done chan struct{}
	val  any
	err  error
}

// Do runs fn for key, or waits for the in-flight call with the same key and
// shares its result. The third return reports whether this caller shared.
func (g *singleflight) Do(key string, fn func() (any, error)) (any, error, bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*flight)
	}
	if f, ok := g.m[key]; ok {
		g.mu.Unlock()
		<-f.done

		return f.val, f.err, true
	}

	f := &flight{done: make(chan struct{})}
	g.m[key] = f
	g.mu.Unlock()

	f.val, f.err = fn()
	close(f.done)

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return f.val, f.err, false
}

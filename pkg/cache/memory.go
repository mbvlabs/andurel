package cache

import (
	"context"
	"time"

	"github.com/maypok86/otter/v2"
)

// entry carries its own deadline so reads stay atomic: no separate
// expire-after call can interleave between the write and the expiry.
type entry struct {
	data      []byte
	expiresAt time.Time
}

func (e entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// counter backs Increment's fixed window.
type counter struct {
	count     int64
	expiresAt time.Time
}

func (c counter) expired(now time.Time) bool {
	return !c.expiresAt.IsZero() && !now.Before(c.expiresAt)
}

// memoryStore is the in-process Store. Values never leave the process; use
// Redis when replicas must share state.
type memoryStore struct {
	namespace string
	values    *otter.Cache[string, entry]
	counters  *otter.Cache[string, counter]
}

var _ Store = (*memoryStore)(nil)

func newMemoryStore(namespace string, maxSize int) *memoryStore {
	return &memoryStore{
		namespace: namespace,
		values:    otter.Must(&otter.Options[string, entry]{MaximumSize: maxSize}),
		counters:  otter.Must(&otter.Options[string, counter]{MaximumSize: maxSize}),
	}
}

func (s *memoryStore) key(key string) string {
	return s.namespace + ":" + key
}

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	stored, ok := s.values.GetIfPresent(s.key(key))
	if !ok || stored.expired(time.Now()) {
		return nil, false, nil
	}

	return stored.data, true, nil
}

func (s *memoryStore) Put(_ context.Context, key string, value []byte, ttl time.Duration) error {
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	s.values.Set(s.key(key), entry{data: value, expiresAt: expiresAt})

	return nil
}

func (s *memoryStore) Forget(_ context.Context, key string) error {
	s.values.Invalidate(s.key(key))

	return nil
}

func (s *memoryStore) Flush(_ context.Context) error {
	s.values.InvalidateAll()
	s.counters.InvalidateAll()

	return nil
}

// Close stops the caches' background goroutines.
func (s *memoryStore) Close() error {
	s.values.StopAllGoroutines()
	s.counters.StopAllGoroutines()

	return nil
}

func (s *memoryStore) Increment(_ context.Context, key string, window time.Duration) (int64, error) {
	var count int64

	s.counters.Compute(s.key(key), func(current counter, found bool) (counter, otter.ComputeOp) {
		if !found || current.expired(time.Now()) {
			count = 1
			return counter{count: 1, expiresAt: time.Now().Add(window)}, otter.WriteOp
		}

		count = current.count + 1

		return counter{count: count, expiresAt: current.expiresAt}, otter.WriteOp
	})

	return count, nil
}

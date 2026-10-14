package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore is the shared Store. Every replica sees the same values and
// rate-limit counters, which is what makes multi-instance deployments behave
// like one.
type redisStore struct {
	namespace string
	client    *redis.Client
}

var _ Store = (*redisStore)(nil)

func newRedisStore(namespace string, cfg RedisConfig) (*redisStore, error) {
	if cfg.Addr == "" {
		return nil, errRedisAddr
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &redisStore{namespace: namespace, client: client}, nil
}

var errRedisAddr = redisConfigError("cache: redis driver needs Config.Redis.Addr")

type redisConfigError string

func (e redisConfigError) Error() string { return string(e) }

func (s *redisStore) key(key string) string {
	return s.namespace + ":" + key
}

func (s *redisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	raw, err := s.client.Get(ctx, s.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}

		return nil, false, err
	}

	return raw, true, nil
}

func (s *redisStore) Put(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return s.client.Set(ctx, s.key(key), value, 0).Err()
	}

	return s.client.Set(ctx, s.key(key), value, ttl).Err()
}

func (s *redisStore) Forget(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

// Flush deletes namespaced keys with a batched SCAN. It never runs FLUSHDB:
// other applications sharing the Redis instance keep their keys.
func (s *redisStore) Flush(ctx context.Context) error {
	var cursor uint64

	for {
		keys, next, err := s.client.Scan(ctx, cursor, s.namespace+":*", 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

// Increment counts a hit inside a fixed window backed by INCR. The first hit
// in a window sets its expiry; INCR is atomic so concurrent replicas each see
// a distinct count.
func (s *redisStore) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	namespaced := s.key(key)

	count, err := s.client.Incr(ctx, namespaced).Result()
	if err != nil {
		return 0, err
	}

	if count == 1 && window > 0 {
		if err := s.client.Expire(ctx, namespaced, window).Err(); err != nil {
			return 0, err
		}
	}

	return count, nil
}

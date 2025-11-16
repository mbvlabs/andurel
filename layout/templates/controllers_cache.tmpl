package controllers

import (
	"context"
	"time"

	"github.com/maypok86/otter/v2"
)

const (
	defaultCacheSize  = 50
	defaultTTL  	  = 15 * time.Minute
)

type Cache[T any] struct {
	cache      *otter.Cache[string, T]
	defaultTTL time.Duration
}

type CacheBuilder[T any] struct {
	size       int
	defaultTTL time.Duration
}

func NewCacheBuilder[T any]() *CacheBuilder[T] {
	return &CacheBuilder[T]{
		size:       defaultCacheSize,
		defaultTTL: defaultTTL,
	}
}

func (b *CacheBuilder[T]) WithSize(size int) *CacheBuilder[T] {
	b.size = size
	return b
}

func (b *CacheBuilder[T]) WithDefaultTTL(ttl time.Duration) *CacheBuilder[T] {
	b.defaultTTL = ttl
	return b
}

func (b *CacheBuilder[T]) Build() (*Cache[T], error) {
	cache := otter.Must(&otter.Options[string, T]{
		MaximumSize: b.size,
	})

	return &Cache[T]{
		cache:      cache,
		defaultTTL: b.defaultTTL,
	}, nil
}

func (c *Cache[T]) Get(key string, loader func() (T, error)) (T, error) {
	if value, ok := c.cache.GetIfPresent(key); ok {
		return value, nil
	}

	result, err := loader()
	if err != nil {
		var zero T
		return zero, err
	}

	c.cache.Set(key, result)
	c.cache.SetExpiresAfter(key, c.defaultTTL)
	return result, nil
}

func (c *Cache[T]) GetIfPresent(ctx context.Context, key string) (T, bool) {
	return c.cache.GetIfPresent(key)
}

func (c *Cache[T]) Set(key string, value T, ttl time.Duration) {
	c.cache.Set(key, value)
	c.cache.SetExpiresAfter(key, ttl)
}

func (c *Cache[T]) SetDefault(key string, value T) {
	c.cache.Set(key, value)
	c.cache.SetExpiresAfter(key, c.defaultTTL)
}

func (c *Cache[T]) Invalidate(key string) {
	c.cache.Invalidate(key)
}

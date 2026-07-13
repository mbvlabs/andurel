// Package cache provides TTL caching for repeated filesystem discovery operations.
package cache

import (
	"sync"
	"time"
)

// Entry stores a cached value and its expiration time.
type Entry struct {
	Value     any
	ExpiresAt time.Time
}

// IsExpired reports whether the entry is past its expiration time.
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// FileSystemCache is a small TTL cache for repeated filesystem lookups.
type FileSystemCache struct {
	entries map[string]*Entry
	mutex   sync.RWMutex
	ttl     time.Duration
}

// NewFileSystemCache creates a new file system cache.
func NewFileSystemCache(ttl time.Duration) *FileSystemCache {
	return &FileSystemCache{
		entries: make(map[string]*Entry),
		ttl:     ttl,
	}
}

// Get returns a cached value when it exists and has not expired.
func (fsc *FileSystemCache) Get(key string) (any, bool) {
	fsc.mutex.RLock()
	defer fsc.mutex.RUnlock()

	entry, exists := fsc.entries[key]
	if !exists || entry.IsExpired() {
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value under key using the cache TTL.
func (fsc *FileSystemCache) Set(key string, value any) {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	fsc.entries[key] = &Entry{
		Value:     value,
		ExpiresAt: time.Now().Add(fsc.ttl),
	}
}

// Delete removes a value from the cache.
func (fsc *FileSystemCache) Delete(key string) {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	delete(fsc.entries, key)
}

// Clear removes all cached entries.
func (fsc *FileSystemCache) Clear() {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	fsc.entries = make(map[string]*Entry)
}

// CleanupExpired removes entries whose TTL has elapsed.
func (fsc *FileSystemCache) CleanupExpired() {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	for key, entry := range fsc.entries {
		if entry.IsExpired() {
			delete(fsc.entries, key)
		}
	}
}

var globalFSCache = NewFileSystemCache(5 * time.Minute)

// GetModulePath returns a cached module path or resolves and caches it.
func GetModulePath(key string, resolver func() (string, error)) (string, error) {
	if cached, found := globalFSCache.Get(key); found {
		return cached.(string), nil
	}

	modulePath, err := resolver()
	if err != nil {
		return "", err
	}

	globalFSCache.Set(key, modulePath)
	return modulePath, nil
}

// GetDirectoryRoot returns a cached directory root or resolves and caches it.
func GetDirectoryRoot(key string, resolver func() (string, error)) (string, error) {
	if cached, found := globalFSCache.Get(key); found {
		return cached.(string), nil
	}

	rootDir, err := resolver()
	if err != nil {
		return "", err
	}

	globalFSCache.Set(key, rootDir)
	return rootDir, nil
}

// GetFileExists returns a cached file existence check or runs and caches it.
func GetFileExists(key string, checker func() bool) bool {
	if cached, found := globalFSCache.Get(key); found {
		return cached.(bool)
	}

	exists := checker()
	globalFSCache.Set(key, exists)
	return exists
}

// ClearFileSystemCache clears file system cache.
func ClearFileSystemCache() {
	globalFSCache.Clear()
}

// CleanupExpiredFileSystemEntries cleans up expired file system entries.
func CleanupExpiredFileSystemEntries() {
	globalFSCache.CleanupExpired()
}

package cache

import (
	"sync"
	"time"
)

type Entry struct {
	Value     interface{}
	ExpiresAt time.Time
}

func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

type FileSystemCache struct {
	entries map[string]*Entry
	mutex   sync.RWMutex
	ttl     time.Duration
}

func NewFileSystemCache(ttl time.Duration) *FileSystemCache {
	return &FileSystemCache{
		entries: make(map[string]*Entry),
		ttl:     ttl,
	}
}

func (fsc *FileSystemCache) Get(key string) (interface{}, bool) {
	fsc.mutex.RLock()
	defer fsc.mutex.RUnlock()

	entry, exists := fsc.entries[key]
	if !exists || entry.IsExpired() {
		return nil, false
	}

	return entry.Value, true
}

func (fsc *FileSystemCache) Set(key string, value interface{}) {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	fsc.entries[key] = &Entry{
		Value:     value,
		ExpiresAt: time.Now().Add(fsc.ttl),
	}
}

func (fsc *FileSystemCache) Delete(key string) {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	delete(fsc.entries, key)
}

func (fsc *FileSystemCache) Clear() {
	fsc.mutex.Lock()
	defer fsc.mutex.Unlock()

	fsc.entries = make(map[string]*Entry)
}

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

func GetFileExists(key string, checker func() bool) bool {
	if cached, found := globalFSCache.Get(key); found {
		return cached.(bool)
	}

	exists := checker()
	globalFSCache.Set(key, exists)
	return exists
}

func ClearFileSystemCache() {
	globalFSCache.Clear()
}

func CleanupExpiredFileSystemEntries() {
	globalFSCache.CleanupExpired()
}
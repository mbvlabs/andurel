package cache

import (
	"errors"
	"testing"
	"time"
)

func TestEntryIsExpired(t *testing.T) {
	if !(&Entry{ExpiresAt: time.Now().Add(-time.Second)}).IsExpired() {
		t.Fatal("expected past entry to be expired")
	}
	if (&Entry{ExpiresAt: time.Now().Add(time.Hour)}).IsExpired() {
		t.Fatal("expected future entry to be fresh")
	}
}

func TestFileSystemCacheLifecycle(t *testing.T) {
	cache := NewFileSystemCache(time.Hour)

	cache.Set("module", "github.com/example/app")
	got, ok := cache.Get("module")
	if !ok || got != "github.com/example/app" {
		t.Fatalf("expected cached module path, got %v, %v", got, ok)
	}

	cache.Delete("module")
	if got, ok := cache.Get("module"); ok || got != nil {
		t.Fatalf("expected deleted key to miss, got %v, %v", got, ok)
	}

	cache.Set("one", 1)
	cache.Set("two", 2)
	cache.Clear()
	if got, ok := cache.Get("one"); ok || got != nil {
		t.Fatalf("expected cleared key to miss, got %v, %v", got, ok)
	}
}

func TestFileSystemCacheExpiration(t *testing.T) {
	cache := NewFileSystemCache(-time.Second)
	cache.Set("expired", "value")

	if got, ok := cache.Get("expired"); ok || got != nil {
		t.Fatalf("expected expired key to miss, got %v, %v", got, ok)
	}

	cache.entries["manual"] = &Entry{
		Value:     "stale",
		ExpiresAt: time.Now().Add(-time.Second),
	}
	cache.CleanupExpired()
	if _, exists := cache.entries["manual"]; exists {
		t.Fatal("expected CleanupExpired to remove stale entry")
	}
}

func TestGlobalFileSystemCacheHelpers(t *testing.T) {
	ClearFileSystemCache()
	t.Cleanup(ClearFileSystemCache)

	calls := 0
	resolver := func() (string, error) {
		calls++
		return "github.com/example/app", nil
	}

	first, err := GetModulePath("module", resolver)
	if err != nil {
		t.Fatalf("GetModulePath failed: %v", err)
	}
	second, err := GetModulePath("module", resolver)
	if err != nil {
		t.Fatalf("GetModulePath failed on cache hit: %v", err)
	}
	if first != second || calls != 1 {
		t.Fatalf("expected one resolver call and stable value, got %q, %q, calls=%d", first, second, calls)
	}

	root, err := GetDirectoryRoot("root", func() (string, error) {
		return "/tmp/project", nil
	})
	if err != nil || root != "/tmp/project" {
		t.Fatalf("expected directory root, got %q, %v", root, err)
	}

	existsCalls := 0
	for range 2 {
		if !GetFileExists("exists", func() bool {
			existsCalls++
			return true
		}) {
			t.Fatal("expected file existence result")
		}
	}
	if existsCalls != 1 {
		t.Fatalf("expected cached checker result, calls=%d", existsCalls)
	}
}

func TestGlobalFileSystemCacheDoesNotCacheErrors(t *testing.T) {
	ClearFileSystemCache()
	t.Cleanup(ClearFileSystemCache)

	expectedErr := errors.New("resolve failed")
	calls := 0
	_, err := GetDirectoryRoot("root-error", func() (string, error) {
		calls++
		return "", expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected resolver error, got %v", err)
	}

	got, err := GetDirectoryRoot("root-error", func() (string, error) {
		calls++
		return "/tmp/project", nil
	})
	if err != nil || got != "/tmp/project" || calls != 2 {
		t.Fatalf("expected retry after error, got %q, %v, calls=%d", got, err, calls)
	}
}

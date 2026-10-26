package anduin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryCounter struct {
	mu     sync.Mutex
	counts map[string]int64
}

func newMemoryCounter() *memoryCounter {
	return &memoryCounter{counts: make(map[string]int64)}
}

func (c *memoryCounter) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counts[key]++

	return c.counts[key], nil
}

func noContent(ctx context.Context) error {
	return NoContent(ctx, http.StatusNoContent)
}

func TestRateLimitPermitsExactlyConfiguredLimitAtomically(t *testing.T) {
	const (
		limit    = 5
		requests = 50
	)

	var handled atomic.Int32
	router := NewRouter()
	router.Post("/limited", func(ctx context.Context) error {
		handled.Add(1)
		return noContent(ctx)
	}, RateLimit(newMemoryCounter(), limit, time.Minute, nil))
	handler := router.Handler()

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() {
			<-start

			request := httptest.NewRequest(http.MethodPost, "/limited", nil)
			request.RemoteAddr = "192.0.2.1:1234"
			handler.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
	close(start)
	wg.Wait()

	if got := handled.Load(); got != limit {
		t.Fatalf("handled requests = %d, want exactly %d", got, limit)
	}
}

func TestRateLimitScopesByRouteAndIP(t *testing.T) {
	const limit = 2

	counter := newMemoryCounter()
	router := NewRouter()
	router.Post("/login", noContent, RateLimit(counter, limit, time.Minute, nil))
	router.Post("/register", noContent, RateLimit(counter, limit, time.Minute, nil))
	handler := router.Handler()

	call := func(path, remoteAddr string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.RemoteAddr = remoteAddr
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		return recorder
	}

	for i := range limit {
		if got := call("/login", "192.0.2.1:1234").Code; got != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want %d", i+1, got, http.StatusNoContent)
		}
	}

	limited := call("/login", "192.0.2.1:1234")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit status = %d, want %d", limited.Code, http.StatusTooManyRequests)
	}
	if got := limited.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want %q", got, "60")
	}

	if got := call("/register", "192.0.2.1:1234").Code; got != http.StatusNoContent {
		t.Fatalf("other route status = %d, want %d", got, http.StatusNoContent)
	}

	if got := call("/login", "192.0.2.2:1234").Code; got != http.StatusNoContent {
		t.Fatalf("second IP status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestRateLimitUsesTrustedProxyClientIP(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}

	router := NewRouter()
	router.Use(RealIP(trusted...))
	router.Post("/login", noContent, RateLimit(newMemoryCounter(), 1, time.Minute, nil))
	handler := router.Handler()

	call := func(forwardedFor string) int {
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		request.RemoteAddr = "10.0.0.1:1234"
		request.Header.Set("X-Forwarded-For", forwardedFor)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		return recorder.Code
	}

	if got := call("203.0.113.1"); got != http.StatusNoContent {
		t.Fatalf("first client status = %d, want %d", got, http.StatusNoContent)
	}
	if got := call("203.0.113.2"); got != http.StatusNoContent {
		t.Fatalf("second client behind same proxy status = %d, want %d", got, http.StatusNoContent)
	}
	if got := call("203.0.113.1"); got != http.StatusTooManyRequests {
		t.Fatalf("repeat client status = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestRateLimitAnswersWithOnLimited(t *testing.T) {
	router := NewRouter()
	router.Post("/login", noContent, RateLimit(newMemoryCounter(), 1, time.Minute,
		func(ctx context.Context) error {
			return Text(ctx, http.StatusTooManyRequests, "slow down")
		},
	))
	handler := router.Handler()

	var recorder *httptest.ResponseRecorder
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		request.RemoteAddr = "192.0.2.1:1234"
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
	}

	if recorder.Code != http.StatusTooManyRequests || recorder.Body.String() != "slow down" {
		t.Fatalf("over-limit response = %d %q, want 429 %q", recorder.Code, recorder.Body.String(), "slow down")
	}
}
